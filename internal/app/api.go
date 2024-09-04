package app

import "ethereum-fetcher/internal/store/pg/models"

//go:generate mockery --name ServiceProvider
type ServiceProvider interface {
	GetUser(username, password string) (*models.User, error)
	GetTransactionsByHashes(txHashes []string, userID int) ([]*models.Transaction, error)
	GetAllTransactions() ([]*models.Transaction, error)
	GetMyTransactions(userID int) ([]*models.Transaction, error)
}
