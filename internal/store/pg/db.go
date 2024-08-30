package pg

import (
	"context"
	"database/sql"
	"fmt"

	"ethereum-fetcher/internal/store/pg/models"

	"github.com/spf13/viper"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
)

type Store struct {
	ctx context.Context
	vp  *viper.Viper
	db  *sql.DB
}

func (st *Store) GetTransactionsByHashes(txHashes []string) ([]*models.Transaction, error) {
	txList, err := models.Transactions(
		qm.SQL(
			fmt.Sprintf(`
				SELECT * FROM %s
				WHERE %s IN (
					SELECT tx_hash FROM unnest($1::TEXT[]) AS t(tx_hash)
				)`,
				models.TableNames.Transactions,
				models.TransactionColumns.TXHash,
			),
			txHashes,
		),
	).All(st.ctx, boil.GetContextDB())
	if err != nil {
		return nil, fmt.Errorf("cannot select tx from database by provided tx hashes: %v", err)
	}

	return txList, nil
}

func (st *Store) InsertTransactions(txList []*models.Transaction) error {
	for _, tx := range txList {
		// Upsert operation for each transaction
		err := tx.Upsert(st.ctx, boil.GetContextDB(), true, []string{models.TransactionColumns.TXHash},
			boil.Infer(), boil.Infer())
		if err != nil {
			return fmt.Errorf("cannot insert tx into the database for hash '%s': %v", tx.TXHash, err)
		}
	}
	return nil
}
