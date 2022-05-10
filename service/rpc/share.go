package rpc

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

const namespacedSharesEndpoint = "/namespaced_shares"

var nIDKey = "nid"

func (h *Handler) handleSharesByNamespaceRequest(w http.ResponseWriter, r *http.Request) {
	// read and parse request
	vars := mux.Vars(r)
	height, err := strconv.Atoi(vars[heightKey])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Errorw("serving request", "endpoint", namespacedSharesEndpoint, "err", err)
		return
	}
	hexNID := vars[nIDKey]
	nID, err := hex.DecodeString(hexNID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Errorw("serving request", "endpoint", namespacedSharesEndpoint, "err", err)
		return
	}
	// get header by given height
	header, err := h.header.GetByHeight(r.Context(), uint64(height))
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
