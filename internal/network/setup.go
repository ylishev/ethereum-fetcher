package network

import (
	"context"
	"fmt"
	"time"

	"ethereum-fetcher/cmd"
	"ethereum-fetcher/internal/store/pg/models"

	"github.com/ethereum/go-ethereum/ethclient"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const maxFetchWorkers = 20

type EthNode struct {
	ctx         context.Context
	vp          *viper.Viper
	client      *ethclient.Client
	rateLimiter *RateLimiter
	workersChan chan struct{}
	tasksChan   chan TxTask
}

func NewEthNode(ctx context.Context, vp *viper.Viper) *EthNode {
	client, err := ethclient.Dial(vp.GetString(cmd.EthNodeURL))
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}

	workersChan := make(chan struct{}, maxFetchWorkers)
	tasksChan := make(chan TxTask)

	node := &EthNode{
		ctx:         ctx,
		vp:          vp,
		client:      client,
		rateLimiter: NewRateLimiter(ctx, vp.GetInt(cmd.NodeRateLimit), time.Second),
		workersChan: workersChan,
		tasksChan:   tasksChan,
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case task := <-tasksChan:
				select {
				// provided context must be a multiplexed version of app context and http request context
				case <-task.Ctx.Done():
					select {
					case task.ResChan <- TxResult{Tx: &models.Transaction{TXHash: task.TxHash}, Err: fmt.Errorf("task canceled")}:
					default:
					}
					close(task.ResChan)
					select {
					case <-ctx.Done():
						return
					default:
						continue // Proceed to the next task in the loop
					}
				case workersChan <- struct{}{}:
					// get permit to work
					go func(task TxTask) {
						defer func() {
							close(task.ResChan)
							// at the end, return the permit, for another worker to obtain it
							<-workersChan
						}()
						log.Infof("start processing task: %s at time %v", task.TxHash, time.Now())

						tx, err := node.GetTransactionByHash(task)
						if tx == nil {
							tx = &models.Transaction{TXHash: task.TxHash}
						}
						log.Infof("completed task: %s at time %v", task.TxHash, time.Now())
						select {
						case <-task.Ctx.Done():
							// Context canceled
							select {
							case task.ResChan <- TxResult{Tx: tx, Err: fmt.Errorf("task canceled")}:
							default:
							}
						case task.ResChan <- TxResult{Tx: tx, Err: err}:
						}
					}(task)
				}
			}
		}
	}()

	return node
}
