package status

import (
	"encoding/json"
	"net/http"
)

type StatusHandler struct {
	Address []string
	Network string
}

func (s StatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//nolint:errcheck
	jsonStatus, err := json.Marshal(s)
	if err != nil {
		jsonStatus, _ = json.Marshal("Error: Could not marshal the status. " + err.Error())
	}

	w.Write(jsonStatus)
}
