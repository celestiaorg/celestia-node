package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/celestiaorg/celestia-node/share"
)

const heightAvailabilityEndpoint = "/data_available"

// AvailabilityResponse represents the response to a
// `/data_available` request.
type AvailabilityResponse struct {
	Available   bool   `json:"available"`
	Probability string `json:"probability_of_availability"`
}

func (h *Handler) handleHeightAvailabilityRequest(w http.ResponseWriter, r *http.Request) {
	heightStr := mux.Vars(r)[heightKey]
	height, err := strconv.Atoi(heightStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, heightAvailabilityEndpoint, err)
		return
	}

	//TODO: change this to NetworkHead once the adjacency in the store is fixed.
	head, err := h.header.LocalHead(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, heightAvailabilityEndpoint, err)
		return
	}
	if headHeight := int(head.Height()); headHeight < height {
		err = fmt.Errorf(
			"current head local chain head: %d is lower than requested height: %d"+
				" give header sync some time and retry later", headHeight, height)
		writeError(w, http.StatusServiceUnavailable, heightAvailabilityEndpoint, err)
		return
	}

	header, err := h.header.GetByHeight(r.Context(), uint64(height))
	if err != nil {
		writeError(w, http.StatusInternalServerError, heightAvailabilityEndpoint, err)
		return
	}

	availResp := &AvailabilityResponse{
		Probability: strconv.FormatFloat(
			h.share.ProbabilityOfAvailability(r.Context()), 'g', -1, 64),
	}

	err = h.share.SharesAvailable(r.Context(), header.DAH)
	switch err {
	case nil:
		availResp.Available = true
		resp, err := json.Marshal(availResp)
		if err != nil {
			writeError(w, http.StatusInternalServerError, heightAvailabilityEndpoint, err)
			return
		}
		_, werr := w.Write(resp)
		if werr != nil {
			log.Errorw("serving request", "endpoint", heightAvailabilityEndpoint, "err", err)
		}
	case share.ErrNotAvailable:
		availResp.Available = false
		resp, err := json.Marshal(availResp)
		if err != nil {
			writeError(w, http.StatusInternalServerError, heightAvailabilityEndpoint, err)
			return
		}
		_, werr := w.Write(resp)
		if werr != nil {
			log.Errorw("serving request", "endpoint", heightAvailabilityEndpoint, "err", err)
		}
	default:
		writeError(w, http.StatusInternalServerError, heightAvailabilityEndpoint, err)
	}
}
