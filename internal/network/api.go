package network

import "ethereum-fetcher/internal/store/pg/models"

//go:generate mockery --name EthereumProvider
type EthereumProvider interface {
	GetTransactionByHash(hash string) (*models.Transaction, error)
}
