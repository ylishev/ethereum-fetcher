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
	httpServer := &http.Server{Addr: ":" + strconv.Itoa(port), Handler: web.router, ReadHeaderTimeout: 5 * time.Second}

	httpServerDone := make(chan struct{})

	go func() {
		select {
		case <-web.ctx.Done():
			// in case of Ctrl+C, shutdown server gracefully
		case <-httpServerDone:
			// the http server is already gone, nothing to do more
			return
		}

		ctx, cancelFunc := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelFunc()
		_ = httpServer.Shutdown(ctx)
	}()
	if err := httpServer.ListenAndServe(); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			close(httpServerDone)
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
		_, err := w.Write(response)
		if err != nil {
			log.Error("cannot write response to the client")
			return
		}
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
