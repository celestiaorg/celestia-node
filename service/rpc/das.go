package rpc

import (
	"encoding/json"
	"net/http"

	"github.com/celestiaorg/celestia-node/das"
)

const (
	dasStateEndpoint = "/daser"
)

// DasStateResponse encompasses the fields returned in response
// to a `/daser` request.
type DasStateResponse struct {
	SampleRoutine  das.RoutineState `json:"sample_routine"`
	CatchUpRoutine das.JobInfo      `json:"catch_up_routine"`
}

func (h *Handler) handleDASStateRequest(w http.ResponseWriter, r *http.Request) {
	dasState := new(DasStateResponse)
	dasState.SampleRoutine = h.das.SampleRoutineState()
	dasState.CatchUpRoutine = h.das.CatchUpRoutineState()

	resp, err := json.Marshal(dasState)
	if err != nil {
		writeError(w, http.StatusInternalServerError, dasStateEndpoint, err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("serving request", "endpoint", dasStateEndpoint, "err", err)
	}
}
