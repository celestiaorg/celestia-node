package rpc

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
)

const namespacedSharesEndpoint = "/namespaced_shares"

// sharesByNamespaceRequest represents a `GetSharesByNamespace`
// request payload
type sharesByNamespaceRequest struct {
	NamespaceID string `json:"namespace_id"`
	Height      uint64 `json:"height"`
}

func (h *Handler) handleSharesByNamespaceRequest(w http.ResponseWriter, r *http.Request) {
	// unmarshal payload
	var req sharesByNamespaceRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Errorw("serving request", "endpoint", namespacedSharesEndpoint, "err", err)
		return
	}
	// decode namespaceID
	nID, err := hex.DecodeString(req.NamespaceID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Errorw("serving request", "endpoint", namespacedSharesEndpoint, "err", err)
		return
	}
	// get header by given height
	header, err := h.header.GetByHeight(r.Context(), req.Height)
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
	resp, err := json.Marshal(shares)
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
