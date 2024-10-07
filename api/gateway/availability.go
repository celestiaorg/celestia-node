package gateway

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/celestiaorg/celestia-node/share"
)

const heightAvailabilityEndpoint = "/data_available"

// AvailabilityResponse represents the response to a
// `/data_available` request.
type AvailabilityResponse struct {
	Available bool `json:"available"`
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

	err = h.share.SharesAvailable(r.Context(), header)
	switch {
	case err == nil:
		resp, err := json.Marshal(&AvailabilityResponse{Available: true})
		if err != nil {
			writeError(w, http.StatusInternalServerError, heightAvailabilityEndpoint, err)
			return
		}
		_, werr := w.Write(resp)
		if werr != nil {
			log.Errorw("serving request", "endpoint", heightAvailabilityEndpoint, "err", err)
		}
	case errors.Is(err, share.ErrNotAvailable):
		resp, err := json.Marshal(&AvailabilityResponse{Available: false})
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
