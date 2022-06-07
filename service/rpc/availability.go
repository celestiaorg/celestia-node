package rpc

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/celestiaorg/celestia-node/service/share"
)

const heightAvailabilityEndpoint = "/data_available"

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
		_, werr := w.Write([]byte(fmt.Sprintf("block data at height %d is available", height)))
		if werr != nil {
			log.Errorw("serving request", "endpoint", heightAvailabilityEndpoint, "err", err)
		}
	case share.ErrNotAvailable:
		_, werr := w.Write([]byte(fmt.Sprintf("block data at height %d is unavailable", height)))
		if werr != nil {
			log.Errorw("serving request", "endpoint", heightAvailabilityEndpoint, "err", err)
		}
	default:
		writeError(w, http.StatusInternalServerError, heightAvailabilityEndpoint, err)
	}
}
