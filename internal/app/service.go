package app

import (
	"context"
	"fmt"

	"ethereum-fetcher/internal/network"
	"ethereum-fetcher/internal/store"
	"ethereum-fetcher/internal/store/pg/models"

	"github.com/spf13/viper"
)

type Service struct {
	ctx context.Context
	vp  *viper.Viper
	st  store.StorageProvider
	net network.EthereumProvider
}

func NewService(ctx context.Context, vp *viper.Viper, st store.StorageProvider, net network.EthereumProvider) *Service {
	return &Service{
		ctx: ctx,
		vp:  vp,
		st:  st,
		net: net,
	}
}

// GetUser get txDB by the provided username and password
func (ap *Service) GetUser(username, password string) (*models.User, error) {
	user, err := ap.st.GetUser(username, password)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetAllTransactions fetches all stored txs in the database
func (ap *Service) GetAllTransactions() ([]*models.Transaction, error) {
	txList, err := ap.st.GetAllTransactions()
	if err != nil {
		return nil, err
	}
	return txList, nil
}

// GetMyTransactions fetches all of my stored txs in the database
func (ap *Service) GetMyTransactions(userID int) ([]*models.Transaction, error) {
	txList, err := ap.st.GetMyTransactions(userID)
	if err != nil {
		return nil, err
	}
	return txList, nil
}

// GetTransactionsByHashes fetches all stored txs in the database by txHashes
func (ap *Service) GetTransactionsByHashes(requestCtx context.Context, txHashes []string, userID int) (
	[]*models.Transaction, error) {
	fullList := make([]*models.Transaction, 0, len(txHashes))

	// fetch stored transactions
	txList, err := ap.st.GetTransactionsByHashes(txHashes, userID)
	if err != nil {
		return nil, err
	}

	// map stored transactions for lookup
	availableMap := make(map[string]*models.Transaction, len(txList))
	for _, tx := range txList {
		availableMap[tx.TXHash] = tx

		// ensure user_transactions table is up-to-date
		if err := ap.st.InsertTransactionsUser([]*models.Transaction{tx}, userID); err != nil {
			return nil, fmt.Errorf("error storing info for hash '%s': %v", tx.TXHash, err)
		}
	}

	muxCtx, cancel := MergeContexts(ap.ctx, requestCtx)
	defer cancel()

	// schedule tasks for missing transactions
	var resultChans []<-chan network.TxResult
	for _, hash := range txHashes {
		if _, found := availableMap[hash]; !found {
			resultChan, err := ap.net.ScheduleTask(muxCtx, hash)
			if err != nil {
				return nil, fmt.Errorf("error scheduling task for hash '%s': %v", hash, err)
			}
			resultChans = append(resultChans, resultChan)
		}
	}

	// process scheduled tasks
	for i := 0; i < len(resultChans); i++ {
		select {
		case result := <-resultChans[i]:
			if result.Err != nil {
				return nil, fmt.Errorf("error fetching task for hash '%s': %v", result.Tx.TXHash, result.Err)
			}
			availableMap[result.Tx.TXHash] = result.Tx
			txList = append(txList, result.Tx)

			// insert newly fetched transactions
			if err := ap.st.InsertTransactions([]*models.Transaction{result.Tx}, userID); err != nil {
				return nil, fmt.Errorf("error storing info for hash '%s': %v", result.Tx.TXHash, err)
			}
		case <-muxCtx.Done():
			return nil, muxCtx.Err()
		}
	}

	// rebuild the result list in the original order of txHashes
	for _, hash := range txHashes {
		if tx, found := availableMap[hash]; found {
			fullList = append(fullList, tx)
		}
	}

	return fullList, nil
}

// MergeContexts provides single context by merging the app context (Ctrl+C handler) and
// http request Context (connection close);
// This goroutine will exit once the request completes (or being canceled);
// Don't use this function if you don't have a way to cancel at least one of them
func MergeContexts(appCtx, requestCtx context.Context) (context.Context, context.CancelFunc) {
	muxCtx, cancel := context.WithCancel(context.Background())

	go func() {
		select {
		case <-appCtx.Done():
			cancel()
		case <-requestCtx.Done():
			cancel()
		}
	}()

	return muxCtx, cancel
}
