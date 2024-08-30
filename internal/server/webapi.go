package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

// WebServer implements endpoint for our API
type WebServer struct {
	ctx              context.Context
	router           *mux.Router
	endPointProvider EndPointProvider
}

// NewServer returns a WebServer object
func NewServer(ctx context.Context, router *mux.Router, endPointProvider EndPointProvider) *WebServer {
	return &WebServer{
		ctx:              ctx,
		router:           router,
		endPointProvider: endPointProvider,
	}
}

// Run method starts the http server
func (web *WebServer) Run(port int) {
	web.endPointProvider.Register(web.router)
	httpServer := &http.Server{Addr: ":" + strconv.Itoa(port), Handler: web.router}
	go func() {
		for {
			select {
			case <-web.ctx.Done():
				ctx, cancelFunc := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancelFunc()
				httpServer.Shutdown(ctx)
				return
			}
		}
	}()
	if err := httpServer.ListenAndServe(); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			log.Error(err)
		}
	}
}

func writeJSONResponse(w http.ResponseWriter, httpCode int, payload interface{}) {
	response, err := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")

	if err != nil {
		// Hijack the response when JSON marshal fails
		httpCode = http.StatusInternalServerError
		response = []byte(`{"error":"Unable to serialize JSON response"}`)

		log.WithFields(log.Fields{
			"error": "json_marshal_response",
		}).Errorf("Unable to marshal response: %s", err.Error())
	}

	w.WriteHeader(httpCode)

	if payload != nil {
		w.Write(response)
	}
}

func writeJSONError(w http.ResponseWriter, httpCode int, err error) {
	writeJSONResponse(w, httpCode, map[string]string{"error": err.Error()})
}

func writeUnauthorizedError(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="restricted", charset="UTF-8"`)
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}

func writeBadRequestError(w http.ResponseWriter) {
	http.Error(w, "Bad Request", http.StatusBadRequest)
}

func writeInternalServerError(w http.ResponseWriter) {
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}

// compile-time check to ensure EndPoint implements the interface
var (
	_ EndPointProvider = &EndPoint{}
)
