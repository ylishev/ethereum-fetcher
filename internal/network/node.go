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

type EthereumProvider interface {
	GetTransactionByHash(hash string) (*models.Transaction, error)
}

type Transaction struct {
	TxHash          string      `json:"transactionHash"`
	TxStatus        int         `json:"transactionStatus"`
	BlockHash       string      `json:"blockHash"`
	BlockNumber     uint64      `json:"blockNumber"`
	From            string      `json:"from"`
	To              null.String `json:"to"`
	ContractAddress null.String `json:"contractAddress"`
	LogsCount       int         `json:"logsCount"`
	Input           string      `json:"input"`
	Value           string      `json:"value"`
}

func (n *EthNode) GetTransactionByHash(hash string) (*models.Transaction, error) {
	var wg sync.WaitGroup

	txHash := common.HexToHash(hash)

	var ethTX *types.Transaction
	var receipt *types.Receipt

	errCh := make(chan error, 2)

	// fetch both requests at the same time
	wg.Add(1)
	go func() {
		defer wg.Done()
		// fetch a transaction by its hash, but obey the rate limitations of the node
		for {
			if n.rateLimiter.Allow() {
				var err error
				ethTX, _, err = n.client.TransactionByHash(n.ctx, txHash)
				if err != nil {
					select {
					case errCh <- fmt.Errorf("failed to fetch transaction details: %v", err):
					case <-n.ctx.Done():
					}
				}
				return
			} else {
				time.Sleep(n.rateLimiter.WaitDuration())
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		// fetch the transaction receipt
		for {
			if n.rateLimiter.Allow() {
				var err error
				receipt, err = n.client.TransactionReceipt(n.ctx, txHash)
				if err != nil {
					select {
					case errCh <- fmt.Errorf("failed to fetch transaction receipt: %v", err):
					case <-n.ctx.Done():
					}
				}
				return
			} else {
				time.Sleep(n.rateLimiter.WaitDuration())
			}
		}
	}()

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
	if n.ctx.Err() != nil && errors.Is(n.ctx.Err(), context.Canceled) {
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

	tx := &models.Transaction{
		TXHash: ethTX.Hash().Hex(),
		// nolint:gosec // handles only the status of the transaction either 1 (success) or 0 (failure)
		TXStatus:        int(receipt.Status),
		BlockHash:       receipt.BlockHash.Hex(),
		BlockNumber:     boilTypes.NewDecimal(bigDec),
		FromAddress:     getTransactionSender(ethTX),
		ToAddress:       toAddress,
		ContractAddress: contractAddress,
		LogsCount:       int64(len(receipt.Logs)),
		Input:           fmt.Sprintf("0x%s", hex.EncodeToString(ethTX.Data())),
		Value:           ethTX.Value().String(),
	}

	return tx, nil
}

// Helper function to get the sender address
func getTransactionSender(tx *types.Transaction) string {
	from, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
	if err != nil {
		log.Fatalf("Failed to get sender: %v", err)
	}
	return from.Hex()
}
