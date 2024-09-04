package server

import (
	"net/http"

	"github.com/gorilla/mux"
)

// EndPointProvider provides access to all endpoint methods available through the web server
//
//go:generate mockery --name EndPointProvider
type EndPointProvider interface {
	GetTransactionsByHashes(w http.ResponseWriter, r *http.Request)
	GetTransactionsByRLP(w http.ResponseWriter, r *http.Request)
	Authenticate(w http.ResponseWriter, r *http.Request)
	Register(router *mux.Router)
}
