package rpc

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/celestiaorg/celestia-node/service/share"
)

const heightAvailabilityEndpoint = "/data_available"

// AvailabilityResponse represents the response to a
// `/data_available` request.
type AvailabilityResponse struct {
	Height      uint64 `json:"height"`
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

	header, err := h.header.GetByHeight(r.Context(), uint64(height))
	if err != nil {
		writeError(w, http.StatusInternalServerError, heightAvailabilityEndpoint, err)
		return
	}

	// calculate the probability of data square availability based
	// on sample size
	probabilityOfAvailability := strconv.FormatFloat(
		share.ProbabilityOfAvailability(share.DefaultSampleAmount), 'g', -1, 64)

	availResp := &AvailabilityResponse{
		Height:      uint64(height),
		Probability: probabilityOfAvailability,
	}

	err = h.share.Availability.SharesAvailable(r.Context(), header.DAH)
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
