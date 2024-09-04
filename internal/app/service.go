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
func (ap *Service) GetTransactionsByHashes(txHashes []string, userID int) ([]*models.Transaction, error) {
	fullList := make([]*models.Transaction, 0, len(txHashes))

	// first grab all stored txs (from the list)
	txList, err := ap.st.GetTransactionsByHashes(txHashes, userID)
	if err != nil {
		return nil, err
	}

	// create a lookup of the returned list of stored txs
	availableMap := make(map[string]int, len(txList))
	for i, v := range txList {
		availableMap[v.TXHash] = i
	}

	// range through all hashes and...
	for _, hash := range txHashes {
		// either take from the already collected query result
		if i, found := availableMap[hash]; found {
			fullList = append(fullList, txList[i])

			// and make sure the join table (user_transactions) has txDB added
			//
			// NOTE: ideally we should only emmit "transactionRead" event
			// and another goroutine to listen and insert the txDB/txs, to separate read and write operations
			err := ap.st.InsertTransactionsUser([]*models.Transaction{txList[i]}, userID)
			if err != nil {
				return nil, fmt.Errorf("error storing txDB txDB info for hash '%s': %v", hash, err)
			}
			continue
		}

		// or otherwise fetch it from the ethereum network
		tx, err := ap.net.GetTransactionByHash(hash)
		if err != nil {
			return nil, fmt.Errorf("error fetching txDB info for hash '%s': %v", hash, err)
		}

		// and then preserve the collected transactions in the store for future access
		err = ap.st.InsertTransactions([]*models.Transaction{tx}, userID)
		if err != nil {
			return nil, fmt.Errorf("error storing txDB info for hash '%s': %v", hash, err)
		}

		fullList = append(fullList, tx)
	}

	return fullList, nil
}
