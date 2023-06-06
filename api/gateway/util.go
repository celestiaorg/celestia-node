package gateway

import (
	"encoding/json"
	"net/http"
)

func writeError(w http.ResponseWriter, statusCode int, endpoint string, err error) {
	log.Debugw("serving request", "endpoint", endpoint, "err", err)

	w.WriteHeader(statusCode)
	errBody, jerr := json.Marshal(err.Error())
	if jerr != nil {
		log.Errorw("serializing error", "endpoint", endpoint, "err", jerr)
		return
	}
	_, werr := w.Write(errBody)
	if werr != nil {
		log.Errorw("writing error response", "endpoint", endpoint, "err", werr)
	}
}

type errResponse struct {
	Data interface{}
	Err  string
}

func writeErrorWithData(w http.ResponseWriter, statusCode int, endpoint string, err error, data interface{}) {
	log.Debugw("serving request", "endpoint", endpoint, "err", err)

	w.WriteHeader(statusCode)

	resp := errResponse{
		Data: data,
		Err:  err.Error(),
	}
	respBody, err := json.Marshal(resp)
	if err != nil {
		log.Errorw("serializing error", "endpoint", endpoint, "err", err)
		return
	}
	_, err = w.Write(respBody)
	if err != nil {
		log.Errorw("writing error response", "endpoint", endpoint, "err", err)
	}
}
