package gateway

import (
	"encoding/json"
	"net/http"
)

const (
	dasStateEndpoint = "/daser/state"
)

func (h *Handler) handleDASStateRequest(w http.ResponseWriter, r *http.Request) {
	stats, err := h.das.SamplingStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, dasStateEndpoint, err)
		return
	}
	resp, err := json.Marshal(stats)
	if err != nil {
		writeError(w, http.StatusInternalServerError, dasStateEndpoint, err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("serving request", "endpoint", dasStateEndpoint, "err", err)
	}
}
