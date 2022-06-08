package rpc

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/celestiaorg/celestia-node/service/share"
)

const heightAvailabilityEndpoint = "/data_available"

// TODO @renaynay: doc
type availabilityResponse struct {
	Height    uint64 `json:"height"`
	Available bool   `json:"available"`
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

	err = h.share.Availability.SharesAvailable(r.Context(), header.DAH)
	switch err {
	case nil:
		resp, err := json.Marshal(&availabilityResponse{
			Height:    uint64(height),
			Available: true,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, heightAvailabilityEndpoint, err)
			return
		}
		_, werr := w.Write(resp)
		if werr != nil {
			log.Errorw("serving request", "endpoint", heightAvailabilityEndpoint, "err", err)
		}
	case share.ErrNotAvailable:
		resp, err := json.Marshal(&availabilityResponse{
			Height:    uint64(height),
			Available: false,
		})
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
