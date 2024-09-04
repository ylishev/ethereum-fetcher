package store

import "ethereum-fetcher/internal/store/pg/models"

// StorageProvider defines the base abstraction around ethereum tx store.
//
//go:generate mockery --name StorageProvider
type StorageProvider interface {
	GetUser(username, password string) (*models.User, error)
	GetTransactionsByHashes(txHashes []string, userID int) ([]*models.Transaction, error)
	GetAllTransactions() ([]*models.Transaction, error)
	GetMyTransactions(userID int) ([]*models.Transaction, error)
	InsertTransactions(txList []*models.Transaction, userID int) error
	InsertTransactionsUser(txList []*models.Transaction, userID int) error
}

const (
	NonAuthenticatedUser int = 0
)
