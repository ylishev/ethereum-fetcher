package store

import "ethereum-fetcher/internal/store/pg/models"

// StorageProvider defines the base abstraction around ethereum tx store.
type StorageProvider interface {
	GetTransactionsByHashes(txHashes []string, userID int) ([]*models.Transaction, error)
	GetAllTransactions() ([]*models.Transaction, error)
	InsertTransactions(txList []*models.Transaction, userID int) error
	InsertTransactionsUser(txList []*models.Transaction, userID int) error
}

const (
	NonAuthenticatedUser int = 0
)
