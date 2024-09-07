package network

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"ethereum-fetcher/internal/store/pg/models"

	"github.com/ericlagergren/decimal"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	log "github.com/sirupsen/logrus"
	"github.com/volatiletech/null/v8"
	boilTypes "github.com/volatiletech/sqlboiler/v4/types"
)

type TxTask struct {
	TxHash  string
	Ctx     context.Context
	ResChan chan TxResult
}

// TxResult is used to transport fetch results over channels
type TxResult struct {
	Tx  *models.Transaction
	Err error
}

// GetTransactionByHash fetch the transaction from the node by provided hash
func (n *EthNode) GetTransactionByHash(task TxTask) (*models.Transaction, error) {
	var wg sync.WaitGroup

	txHash := common.HexToHash(task.TxHash)

	var ethTX *types.Transaction
	var receipt *types.Receipt

	errCh := make(chan error, 2)

	// fetch both requests at the same time
	wg.Add(2)

	n.fetchTransactionDetails(task.Ctx, &wg, &ethTX, txHash, errCh)
	n.fetchTransactionReceipt(task.Ctx, &wg, &receipt, txHash, errCh)

	// close the error channel when both goroutines are done
	go func() {
		wg.Wait()
		close(errCh)
	}()

	// handle multiple errors as one
	var errList []error
	for err := range errCh {
		if err != nil {
			errList = append(errList, err)
		}
	}

	// upon context cancellation, quit here the execution with abort message
	if task.Ctx.Err() != nil && errors.Is(task.Ctx.Err(), context.Canceled) {
		return nil, errors.New("fetching ethereum tx data got interrupted by context cancellation")
	}

	if len(errList) > 0 {
		// sort the errors list, for consistency, since both goroutines might return in random order
		slices.SortFunc(errList, func(a, b error) int {
			return strings.Compare(a.Error(), b.Error())
		})
		return nil, errors.Join(errList...)
	}

	var toAddress null.String
	if addr := ethTX.To(); addr != nil && *addr != (common.Address{}) {
		toAddress = null.StringFrom(ethTX.To().Hex())
	}

	var contractAddress null.String
	if receipt.ContractAddress != (common.Address{}) {
		contractAddress = null.StringFrom(receipt.ContractAddress.Hex())
	}
	bigDec := new(decimal.Big)
	bigDec.SetBigMantScale(receipt.BlockNumber, 0)

	fromAddress, err := getTransactionSender(ethTX)
	if err != nil {
		return nil, fmt.Errorf("error fetching ethereum tx data: %v", err)
	}

	tx := &models.Transaction{
		TXHash: ethTX.Hash().Hex(),
		// nolint:gosec // handles only the status of the transaction either 1 (success) or 0 (failure)
		TXStatus:        int(receipt.Status),
		BlockHash:       receipt.BlockHash.Hex(),
		BlockNumber:     boilTypes.NewDecimal(bigDec),
		FromAddress:     fromAddress,
		ToAddress:       toAddress,
		ContractAddress: contractAddress,
		LogsCount:       int64(len(receipt.Logs)),
		Input:           fmt.Sprintf("0x%s", hex.EncodeToString(ethTX.Data())),
		Value:           ethTX.Value().String(),
	}

	return tx, nil
}

func (n *EthNode) fetchTransactionReceipt(ctx context.Context, wg *sync.WaitGroup, receipt **types.Receipt,
	txHash common.Hash, errCh chan error) {
	go func() {
		defer wg.Done()
		// fetch the transaction receipt
		for {
			if n.rateLimiter.Allow() {
				var err error
				*receipt, err = n.client.TransactionReceipt(ctx, txHash)
				if err != nil {
					select {
					case errCh <- fmt.Errorf("failed to fetch transaction receipt: %v", err):
					case <-ctx.Done():
					}
				}
				return
			}
			time.Sleep(n.rateLimiter.WaitDuration())
		}
	}()
}

func (n *EthNode) fetchTransactionDetails(ctx context.Context, wg *sync.WaitGroup, ethTX **types.Transaction,
	txHash common.Hash, errCh chan error) {
	go func() {
		defer wg.Done()
		// fetch a transaction by its hash, but obey the rate limitations of the node
		for {
			if n.rateLimiter.Allow() {
				var err error
				*ethTX, _, err = n.client.TransactionByHash(ctx, txHash)
				if err != nil {
					select {
					case errCh <- fmt.Errorf("failed to fetch transaction details: %v", err):
					case <-ctx.Done():
					}
				}
				return
			}
			time.Sleep(n.rateLimiter.WaitDuration())
		}
	}()
}

func (n *EthNode) ScheduleTask(muxCtx context.Context, txHash string) (<-chan TxResult, error) {
	resChan := make(chan TxResult)

	task := TxTask{
		TxHash:  txHash,
		Ctx:     muxCtx,
		ResChan: resChan,
	}

	select {
	case n.tasksChan <- task:
	case <-muxCtx.Done():
		close(resChan)
		return resChan, fmt.Errorf("request canceled, error fetching info for hash '%s'", txHash)
	}

	return resChan, nil
}

// getTransactionSender function to get the sender address
func getTransactionSender(tx *types.Transaction) (string, error) {
	from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
	if err != nil {
		log.Errorf("failed to get sender: %v", err)
		return "", err
	}
	return from.Hex(), nil
}
