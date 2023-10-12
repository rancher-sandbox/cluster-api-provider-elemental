package api

import (
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
)

// WriteResponse is the same wrapper as WriteResponseBytes, but accepts []byte as input.
func WriteResponse(logger logr.Logger, writer http.ResponseWriter, response string) {
	WriteResponseBytes(logger, writer, []byte(response))
}

// WriteResponseBytes is a wrapper to handle HTTP response writing errors with logging.
func WriteResponseBytes(logger logr.Logger, writer http.ResponseWriter, response []byte) {
	if n, err := fmt.Fprint(writer, response); err != nil {
		logger.Error(err, fmt.Sprintf("Could not write response, written %d bytes", n))
	}
}
