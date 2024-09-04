package server

import (
	"context"
	"net/http"

	"ethereum-fetcher/cmd"
	"ethereum-fetcher/internal/app"

	"github.com/gorilla/mux"
	"github.com/spf13/viper"
)

// EndPoint provides endpoints with access to the shared resources via DI
type EndPoint struct {
	ctx context.Context
	vp  *viper.Viper
	ap  app.ServiceProvider
}

// NewEndPoint returns a EndPoint object that provides endpoints and shared resources
func NewEndPoint(ctx context.Context, vp *viper.Viper, ap app.ServiceProvider) *EndPoint {
	return &EndPoint{
		ctx: ctx,
		vp:  vp,
		ap:  ap,
	}
}

// Register endpoints with the router
func (ep *EndPoint) Register(router *mux.Router) {
	jwtSecret := ep.vp.GetString(cmd.JWTSecret)
	router.HandleFunc("/lime/eth",
		NewAuthBearerMiddleware(jwtSecret, ep.GetTransactionsByHashes, true).Authenticate).Methods("GET")
	router.HandleFunc("/lime/eth/{rlphex}",
		NewAuthBearerMiddleware(jwtSecret, ep.GetTransactionsByRLP, true).Authenticate).Methods("GET")
	router.HandleFunc("/lime/all", ep.GetAllTransactions).Methods("GET")
	router.HandleFunc("/lime/my",
		NewAuthBearerMiddleware(jwtSecret, ep.GetMyTransactions, false).Authenticate).Methods("GET")
	router.HandleFunc("/lime/authenticate", ep.Authenticate).Methods("POST")

	router.NotFoundHandler = http.HandlerFunc(NotImplemented)
}

// compile-time check to ensure EndPoint implements the interface
var (
	_ EndPointProvider = &EndPoint{}
)
