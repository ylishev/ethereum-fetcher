package store

import "ethereum-fetcher/internal/store/pg/models"

// StoreProvider defines the base abstraction around ethereum tx store.
type StoreProvider interface {
	GetTransactionsByHashes(txHashes []string, userID int) ([]*models.Transaction, error)
	InsertTransactions(txList []*models.Transaction, userID int) error
	InsertTransactionsUser(txList []*models.Transaction, userID int) error
}
