package gateway

import (
	"net/http"
)

func writeError(w http.ResponseWriter, statusCode int, endpoint string, err error) {
	log.Debugw("serving request", "endpoint", endpoint, "err", err)

	w.WriteHeader(statusCode)

	errorMessage := err.Error() // Get the error message as a string
	errorBytes := []byte(errorMessage)

	_, err = w.Write(errorBytes)
	if err != nil {
		log.Errorw("writing error response", "endpoint", endpoint, "err", err)
	}
}
