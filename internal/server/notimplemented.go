package server

import (
	"errors"
	"net/http"
)

// NotImplemented default error handler
func NotImplemented(w http.ResponseWriter, _ *http.Request) {
	writeJSONError(w, http.StatusNotImplemented, errors.New("not implemented"))
}
