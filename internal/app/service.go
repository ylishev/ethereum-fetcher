package app

import (
	"context"
	"fmt"

	"ethereum-fetcher/internal/network"
	"ethereum-fetcher/internal/store"
	"ethereum-fetcher/internal/store/pg/models"

	"github.com/spf13/viper"
)

type ServiceProvider interface {
	GetTransactionsByHashes(txHashes []string, userID int) ([]*models.Transaction, error)
	GetAllTransactions() ([]*models.Transaction, error)
}

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

func (ap *Service) GetAllTransactions() ([]*models.Transaction, error) {
	// fetch all stored tx from the database
	txList, err := ap.st.GetAllTransactions()
	if err != nil {
		return nil, err
	}
	return txList, nil
}

func (ap *Service) GetTransactionsByHashes(txHashes []string, userID int) ([]*models.Transaction, error) {
	fullList := make([]*models.Transaction, 0, len(txHashes))

	// fetch all stored tx from the database by those hashes
	txList, err := ap.st.GetTransactionsByHashes(txHashes, userID)
	if err != nil {
		return nil, err
	}

	// map all selected tx from db to the index of txList
	availableMap := make(map[string]int, len(txList))
	for i, v := range txList {
		availableMap[v.TXHash] = i
	}

	// range through all hashes and get the tx either from db results set or get it from the ethereum node
	for _, hash := range txHashes {
		// try first from the query result
		if i, found := availableMap[hash]; found {
			fullList = append(fullList, txList[i])

			// now make sure user_transactions has the user added
			// NOTE: ideally we should only emmit "transactionRead" event
			// and another goroutine to listen and insert the user/txs, to separate read and write operations
			err := ap.st.InsertTransactionsUser([]*models.Transaction{txList[i]}, userID)
			if err != nil {
				return nil, fmt.Errorf("error storing tx user info for hash '%s': %v", hash, err)
			}
			continue
		}

		// otherwise fetch it from the ethereum network
		tx, err := ap.net.GetTransactionByHash(hash)
		if err != nil {
			return nil, fmt.Errorf("error fetching tx info for hash '%s': %v", hash, err)
		}

		// NOTE: ideally we should only emmit "transactionRead" event
		// and another goroutine to listen and insert the tx, to separate read and write operations
		err = ap.st.InsertTransactions([]*models.Transaction{tx}, userID)
		if err != nil {
			return nil, fmt.Errorf("error storing tx info for hash '%s': %v", hash, err)
		}

		fullList = append(fullList, tx)
	}

	return fullList, nil
}
