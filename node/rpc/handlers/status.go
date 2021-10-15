package handlers

import (
	"encoding/json"
	"net/http"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("RPC")

type StatusMessage struct {
	ListenAddresses []string
	Network         string
}

func NewStatusMessage(listenAddrs []string, network string) *StatusMessage {
	return &StatusMessage{
		ListenAddresses: listenAddrs,
		Network:         network,
	}
}

func (s StatusMessage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	jsonStatus, err := json.Marshal(s)
	if err != nil {
		log.Errorf("could not marshal the status: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		_, err = w.Write(jsonStatus)
		if err != nil {
			log.Errorf("could not write status response: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}
