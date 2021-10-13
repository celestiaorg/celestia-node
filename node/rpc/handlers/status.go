package handlers

import (
	"encoding/json"
	"net/http"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("status")

type StatusHandler struct {
	Address []string
	Network string
}

func (s StatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	jsonStatus, err := json.Marshal(s)
	if err != nil {
		errorMessage := "Could not marshal the status. " + err.Error()
		jsonStatus, _ = json.Marshal(errorMessage)
		log.Error(errorMessage)
	}

	_, err = w.Write(jsonStatus)
	if err != nil {
		log.Error("Could not write status response. " + err.Error())
	}
}
