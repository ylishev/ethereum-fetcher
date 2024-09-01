package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

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

	var queryMods []qm.QueryMod
	switch userID == 0 {
	// no authenticated user, just load all records by hash value
	case true:
		baseQuery := `
        SELECT %s FROM %s t
        WHERE t.%s IN (
            SELECT LOWER(tx_hash) FROM unnest($1::TEXT[]) AS hashes(tx_hash)
        )
    `
		queryMods = []qm.QueryMod{
			qm.SQL(
				fmt.Sprintf(baseQuery,
					columns,
					models.TableNames.Transactions,
					models.TransactionColumns.TXHash,
				),
				txHashes,
			),
		}
	case false:
		// we have authenticated user, so do
		baseQueryWithJoin := `
            SELECT %s FROM %s t
            LEFT JOIN %s ut ON t.%s = ut.%s
            WHERE ut.%s = $2 AND t.%s IN (
                SELECT tx_hash FROM unnest($1::TEXT[]) AS hashes(tx_hash)
            )
        `
		queryMods = []qm.QueryMod{
			qm.SQL(
				fmt.Sprintf(baseQueryWithJoin,
					columns,
					models.TableNames.Transactions,
					models.TableNames.UserTransactions,
					models.TransactionColumns.TXHash,
					models.TransactionColumns.TXHash,
					`user_id`,
					models.TransactionColumns.TXHash,
				),
				txHashes,
				userID,
			),
			qm.Load(qm.Rels(models.TransactionRels.Users), qm.Where("user_id = ?", userID)),
		}
	}

	txList, err := models.Transactions(queryMods...).All(st.ctx, boil.GetContextDB())
	if err != nil {
		return nil, fmt.Errorf("cannot select tx from database by provided tx hashes: %v", err)
	}

	return txList, nil
}

func (st *Store) InsertTransactions(txList []*models.Transaction, userID int) error {
	for _, tx := range txList {
		// Upsert operation for each ethereum transaction

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

		if userID > 0 {
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
		// Add user/txs to the join table
		if userID > 0 {
			user := &models.User{
				ID: userID,
			}
			/*// first try to load users for the transaction (filter by userID)
			err := tx.L.LoadUsers(st.ctx, boil.GetContextDB(), true, tx, qm.Where("user_id = ?", userID))
			if err != nil {
				return fmt.Errorf("cannot load tx/user from the database for hash '%s': %v", tx.TXHash, err)
			}*/
			// and in case current user is not yet added - do it now
			if len(tx.R.GetUsers()) == 0 {
				err := tx.AddUsers(st.ctx, boil.GetContextDB(), false, user)
				if err != nil {
					if pgErr := new(pgconn.PgError); !errors.As(err, &pgErr) || pgErr.Code != ErrDuplicateRowCode {
						return fmt.Errorf("cannot insert tx/user into the database for hash '%s': %v", tx.TXHash, err)
					}
				}
			}

			/*_, err := tx.Users(qm.Where("user_id = ?", userID)).One(st.ctx, boil.GetContextDB())
			if err != nil {
				if !errors.Is(err, sql.ErrNoRows) {
					return fmt.Errorf("cannot insert tx/user into the database for hash '%s': %v", tx.TXHash, err)
				}
				err := tx.AddUsers(st.ctx, boil.GetContextDB(), false, user)
				if err != nil {
					return fmt.Errorf("cannot insert tx/user into the database for hash '%s': %v", tx.TXHash, err)
				}
			}*/

		}
	}
	return nil
}
