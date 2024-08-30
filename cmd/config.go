package cmd

import (
	"strconv"

	"github.com/spf13/viper"
)

const (
	APIPort           = "APIPort"
	EthNodeURL        = "EthNodeURL"
	DBConnectionURL   = "DBConnectionURL"
	JWTSecret         = "JWTSecret"
	LogLevel          = "LogLevel"
	NodeRateLimit     = "NodeRateLimit"
	DefaultNodeCredit = 10
)

func NewViper() *viper.Viper {
	vp := viper.New()
	vp.AutomaticEnv()

	_ = vp.BindEnv(APIPort, "API_PORT")
	_ = vp.BindEnv(EthNodeURL, "ETH_NODE_URL")
	_ = vp.BindEnv(DBConnectionURL, "DB_CONNECTION_URL")
	_ = vp.BindEnv(JWTSecret, "JWT_SECRET")
	_ = vp.BindEnv(LogLevel, "LOG_LEVEL")
	_ = vp.BindEnv(NodeRateLimit, "NODE_RATE_LIMIT_PER_SECOND")

	vp.SetDefault(LogLevel, "info")
	vp.SetDefault(NodeRateLimit, strconv.Itoa(DefaultNodeCredit))

	return vp
}
