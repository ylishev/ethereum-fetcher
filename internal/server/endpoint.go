package server

import (
	"context"
	"net/http"

	"ethereum-fetcher/cmd"
	"ethereum-fetcher/internal/app"
	"ethereum-fetcher/internal/store"

	"github.com/gorilla/mux"
	"github.com/spf13/viper"
)

// EndPointProvider provides access to all endpoint methods available through the web server
type EndPointProvider interface {
	GetTransactionsByHashes(w http.ResponseWriter, r *http.Request)
	GetTransactionsByRLP(w http.ResponseWriter, r *http.Request)
	Authenticate(w http.ResponseWriter, r *http.Request)
	Register(router *mux.Router)
}

// EndPoint provides endpoints with access to the shared resources via DI.
type EndPoint struct {
	ctx context.Context
	vp  *viper.Viper
	st  store.StorageProvider
	ap  app.ServiceProvider
}

// NewEndPoint returns a EndPoint object that provides endpoints and shared resources.
func NewEndPoint(ctx context.Context, vp *viper.Viper, ap app.ServiceProvider, st store.StorageProvider) *EndPoint {
	return &EndPoint{
		ctx: ctx,
		vp:  vp,
		ap:  ap,
		st:  st,
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
		NewAuthBearerMiddleware(jwtSecret, ep.GetTransactionsByRLP, false).Authenticate).Methods("GET")
	router.HandleFunc("/lime/authenticate", ep.Authenticate).Methods("POST")

	router.NotFoundHandler = http.HandlerFunc(NotImplemented)
}
