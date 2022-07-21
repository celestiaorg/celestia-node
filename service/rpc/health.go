package rpc

import (
	"encoding/json"
	"net/http"
)

const healthEndpoint = "/health"

// HealthResponse represents the response to a
// `/health` request.
type HealthResponse struct {
	Status string `json:"status"`
}

func (h *Handler) handleHealthRequest(w http.ResponseWriter, r *http.Request) {
	availResp := &HealthResponse{
		Status: "ok",
	}

	resp, err := json.Marshal(availResp)
	if err != nil {
		writeError(w, http.StatusInternalServerError, healthEndpoint, err)
		return
	}
	_, werr := w.Write(resp)
	if werr != nil {
		log.Errorw("serving request", "endpoint", healthEndpoint, "err", err)
	}
}
