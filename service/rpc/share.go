package rpc

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/service/share"
)

const namespacedSharesEndpoint = "/namespaced_shares"

var nIDKey = "nid"

// NamespacedSharesResponse represents the response to a
// SharesByNamespace request.
type NamespacedSharesResponse struct {
	Shares []share.Share `json:"shares"`
	Height uint64        `json:"height"`
}

func (h *Handler) handleSharesByNamespaceRequest(w http.ResponseWriter, r *http.Request) {
	// read and parse request
	vars := mux.Vars(r)
	var (
		height = 0
		err    error
	)
	// if a height was given, parse it, otherwise get namespaced shares
	// from the latest header
	if strHeight, ok := vars[heightKey]; ok {
		height, err = strconv.Atoi(strHeight)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			log.Errorw("serving request", "endpoint", namespacedSharesEndpoint, "err", err)
			return
		}
	}
	hexNID := vars[nIDKey]
	nID, err := hex.DecodeString(hexNID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Errorw("serving request", "endpoint", namespacedSharesEndpoint, "err", err)
		return
	}
	// get header
	var header *header.ExtendedHeader
	switch height {
	case 0:
		header, err = h.header.Head(r.Context())
	default:
		header, err = h.header.GetByHeight(r.Context(), uint64(height))
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorw("serving request", "endpoint", namespacedSharesEndpoint, "err", err)
		return
	}
	// perform request
	shares, err := h.share.GetSharesByNamespace(r.Context(), header.DAH, nID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, werr := w.Write([]byte(err.Error()))
		if werr != nil {
			log.Errorw("writing response", "endpoint", namespacedSharesEndpoint, "err", werr)
		}
		log.Errorw("serving request", "endpoint", namespacedSharesEndpoint, "err", err)
		return
	}
	resp, err := json.Marshal(&NamespacedSharesResponse{
		Shares: shares,
		Height: uint64(header.Height),
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorw("serving request", "endpoint", namespacedSharesEndpoint, "err", err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("serving request", "endpoint", namespacedSharesEndpoint, "err", err)
	}
}
