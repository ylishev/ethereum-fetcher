package network

import (
	"context"

	"ethereum-fetcher/internal/store/pg/models"
)

//go:generate mockery --name EthereumProvider
type EthereumProvider interface {
	GetTransactionByHash(task TxTask) (*models.Transaction, error)
	ScheduleTask(muxCtx context.Context, txHash string) (<-chan TxResult, error)
}
