package rpc

import (
	"encoding/json"
	"net/http"
)

type JSONError struct {
	Err string `json:"error"`
}

func writeError(w http.ResponseWriter, statusCode int, endpoint string, err error) {
	log.Errorw("serving request", "endpoint", endpoint, "err", err)

	w.WriteHeader(statusCode)
	errBody, jerr := json.Marshal(JSONError{Err: err.Error()})
	if jerr != nil {
		log.Errorw("serializing error", "endpoint", endpoint, "err", jerr)
		return
	}
	_, werr := w.Write(errBody)
	if werr != nil {
		log.Errorw("writing error response", "endpoint", endpoint, "err", werr)
	}
}
