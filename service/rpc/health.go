package rpc

import (
	"encoding/json"
	"net/http"
)

const healthEndpoint = "/health"

// HealthResponse represents the response to a
// `/health` request.
type HealthResponse struct {
	StateService  bool             `json:"state_service"`
	ShareService  bool             `json:"share_service"`
	DASStatus     DasStateResponse `json:"das_status"`
	HeaderSyncing bool             `json:"header_syncing"`
}

func (h *Handler) handleHealthRequest(w http.ResponseWriter, r *http.Request) {
	availResp := &HealthResponse{
		StateService:  !h.state.IsStopped(),
		ShareService:  !h.share.IsStopped(),
		HeaderSyncing: h.header.IsSyncing(),
	}

	if h.das != nil {
		dasState := new(DasStateResponse)
		dasState.SampleRoutine = h.das.SampleRoutineState()
		dasState.CatchUpRoutine = h.das.CatchUpRoutineState()
		availResp.DASStatus = *dasState
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
