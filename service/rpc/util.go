package rpc

import (
	"encoding/json"
	"net/http"
)

func writeError(w http.ResponseWriter, statusCode int, endpoint string, err error) {
	w.WriteHeader(statusCode)
	errBody, jerr := json.Marshal(err.Error())
	if jerr != nil {
		log.Errorw("serializing error", "endpoint", endpoint, "err", jerr)
		return
	}
	_, werr := w.Write(errBody)
	if werr != nil {
		log.Errorw("writing response", "endpoint", endpoint, "err", werr)
	}
	log.Errorw("serving request", "endpoint", endpoint, "err", err)
}
