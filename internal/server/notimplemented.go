package server

import (
	"errors"
	"net/http"
)

// NotImplemented default error handler
func NotImplemented(w http.ResponseWriter, r *http.Request) {
	writeJSONError(w, http.StatusNotImplemented, errors.New("not implemented"))
}
