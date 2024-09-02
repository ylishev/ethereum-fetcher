package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"ethereum-fetcher/internal/store"
	"ethereum-fetcher/internal/store/pg/models"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/spf13/viper"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

type Store struct {
	ctx context.Context
	vp  *viper.Viper
	db  *sql.DB
}

const ErrDuplicateRowCode = "23505"
const MaxAllResults = 1000

func (st *Store) GetAllTransactions() ([]*models.Transaction, error) {
	// limit the result set to the MaxAllResults, but if there is a requirement to support "unlimited"
	// it is better to have either a pagination or a PG CURSOR + FETCH + golang chan to stream the results back
	txList, err := models.Transactions(
		qm.Limit(MaxAllResults),
	).All(st.ctx, boil.GetContextDB())
	if err != nil {
		return nil, fmt.Errorf("cannot select all tx from database: %v", err)
	}

	return txList, nil
}

func (st *Store) GetTransactionsByHashes(txHashes []string, userID int) ([]*models.Transaction, error) {
	columns := strings.Join([]string{
		"t." + models.TransactionColumns.TXHash,
		"t." + models.TransactionColumns.TXStatus,
		"t." + models.TransactionColumns.BlockHash,
		"t." + models.TransactionColumns.BlockNumber,
		"t." + models.TransactionColumns.FromAddress,
		"t." + models.TransactionColumns.ToAddress,
		"t." + models.TransactionColumns.ContractAddress,
		"t." + models.TransactionColumns.LogsCount,
		"t." + models.TransactionColumns.Input,
		"t." + models.TransactionColumns.Value,
	}, ", ")

	baseQuery := `
        SELECT %s FROM %s t
        WHERE t.%s IN (
            SELECT LOWER(tx_hash) FROM unnest($1::TEXT[]) AS hashes(tx_hash)
        )
    `

	queryMods := []qm.QueryMod{
		qm.SQL(
			fmt.Sprintf(baseQuery,
				columns,
				models.TableNames.Transactions,
				models.TransactionColumns.TXHash,
			),
			txHashes,
		),
	}

	if userID > store.NonAuthenticatedUser {
		// load only the authenticated user for those transactions
		queryMods = append(queryMods,
			qm.Load(qm.Rels(models.TransactionRels.Users), qm.Where("user_id = ?", userID)),
		)
	}

	txList, err := models.Transactions(queryMods...).All(st.ctx, boil.GetContextDB())
	if err != nil {
		return nil, fmt.Errorf("cannot select tx from database by provided tx hashes: %v", err)
	}

	return txList, nil
}

func (st *Store) InsertTransactions(txList []*models.Transaction, userID int) error {
	for _, tx := range txList {
		// upsert operation for each ethereum transaction

		// first start dbTX, to ensure that both eth TX and user/TX are inserted
		dbTx, err := boil.BeginTx(st.ctx, nil)
		if err != nil {
			return fmt.Errorf("cannot insert tx into the database for hash '%s': %v", tx.TXHash, err)
		}

		err = tx.Upsert(st.ctx, dbTx, true, []string{models.TransactionColumns.TXHash},
			boil.Infer(), boil.Infer())
		if err != nil {
			_ = dbTx.Rollback()
			return fmt.Errorf("cannot insert tx into the database for hash '%s': %v", tx.TXHash, err)
		}

		if userID > store.NonAuthenticatedUser {
			user := &models.User{
				ID: userID,
			}
			err := tx.AddUsers(st.ctx, dbTx, false, user)
			if err != nil {
				_ = dbTx.Rollback()
				return fmt.Errorf("cannot insert tx/user into the database for hash '%s': %v", tx.TXHash, err)
			}
		}
		err = dbTx.Commit()
		if err != nil {
			return fmt.Errorf("cannot insert tx into the database for hash '%s': %v", tx.TXHash, err)
		}
	}

	return nil
}

func (st *Store) InsertTransactionsUser(txList []*models.Transaction, userID int) error {
	for _, tx := range txList {
		// add user/txs to the join table
		if userID > store.NonAuthenticatedUser {
			user := &models.User{
				ID: userID,
			}
			// and in case current user is not yet added - do it now
			if len(tx.R.GetUsers()) == 0 {
				err := tx.AddUsers(st.ctx, boil.GetContextDB(), false, user)
				if err != nil {
					if pgErr := new(pgconn.PgError); !errors.As(err, &pgErr) || pgErr.Code != ErrDuplicateRowCode {
						return fmt.Errorf("cannot insert tx/user into the database for hash '%s': %v", tx.TXHash, err)
					}
				}
			}
		}
	}
	return nil
}
