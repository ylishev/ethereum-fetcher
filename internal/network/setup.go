package network

import (
	"context"
	"time"

	"ethereum-fetcher/cmd"

	"github.com/ethereum/go-ethereum/ethclient"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type EthNode struct {
	ctx         context.Context
	vp          *viper.Viper
	client      *ethclient.Client
	rateLimiter *RateLimiter
}

func NewEthNode(ctx context.Context, vp *viper.Viper) *EthNode {
	client, err := ethclient.Dial(vp.GetString(cmd.EthNodeURL))
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}

	return &EthNode{
		ctx:         ctx,
		vp:          vp,
		client:      client,
		rateLimiter: NewRateLimiter(ctx, vp.GetInt(cmd.NodeRateLimit), time.Second),
	}
}
