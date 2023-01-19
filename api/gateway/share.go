package gateway

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	appshares "github.com/celestiaorg/celestia-app/pkg/shares"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/nmt/namespace"
)

const (
	namespacedSharesEndpoint = "/namespaced_shares"
	namespacedDataEndpoint   = "/namespaced_data"
)

var nIDKey = "nid"

// NamespacedSharesResponse represents the response to a
// SharesByNamespace request.
type NamespacedSharesResponse struct {
	Shares []share.Share `json:"shares"`
	Height uint64        `json:"height"`
}

// NamespacedDataResponse represents the response to a
// DataByNamespace request.
type NamespacedDataResponse struct {
	Data   [][]byte `json:"data"`
	Height uint64   `json:"height"`
}

func (h *Handler) handleSharesByNamespaceRequest(w http.ResponseWriter, r *http.Request) {
	height, nID, err := parseGetByNamespaceArgs(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, namespacedSharesEndpoint, err)
		return
	}
	shares, headerHeight, err := h.getShares(r.Context(), height, nID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, namespacedSharesEndpoint, err)
		return
	}
	resp, err := json.Marshal(&NamespacedSharesResponse{
		Shares: shares,
		Height: uint64(headerHeight),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, namespacedSharesEndpoint, err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("serving request", "endpoint", namespacedSharesEndpoint, "err", err)
	}
}

func (h *Handler) handleDataByNamespaceRequest(w http.ResponseWriter, r *http.Request) {
	height, nID, err := parseGetByNamespaceArgs(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, namespacedDataEndpoint, err)
		return
	}
	shares, headerHeight, err := h.getShares(r.Context(), height, nID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, namespacedDataEndpoint, err)
		return
	}
	data, err := dataFromShares(shares)
	if err != nil {
		writeError(w, http.StatusInternalServerError, namespacedDataEndpoint, err)
		return
	}
	resp, err := json.Marshal(&NamespacedDataResponse{
		Data:   data,
		Height: uint64(headerHeight),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, namespacedDataEndpoint, err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("serving request", "endpoint", namespacedDataEndpoint, "err", err)
	}
}

func (h *Handler) getShares(ctx context.Context, height uint64, nID namespace.ID) ([]share.Share, int64, error) {
	// get header
	var (
		err    error
		header *header.ExtendedHeader
	)
	switch height {
	case 0:
		header, err = h.header.Head(ctx)
	default:
		header, err = h.header.GetByHeight(ctx, height)
	}
	if err != nil {
		return nil, 0, err
	}
	// perform request
	shares, err := h.share.GetSharesByNamespace(ctx, header.DAH, nID)
	return shares.Flatten(), header.Height, err
}

func dataFromShares(shares []share.Share) ([][]byte, error) {
	blobs, err := appshares.ParseBlobs(shares)
	if err != nil {
		return nil, err
	}
	data := make([][]byte, len(blobs))
	for i := range blobs {
		data[i] = blobs[i].Data
	}
	return data, nil
}

func parseGetByNamespaceArgs(r *http.Request) (height uint64, nID namespace.ID, err error) {
	vars := mux.Vars(r)
	// if a height was given, parse it, otherwise get namespaced shares/data from the latest header
	if strHeight, ok := vars[heightKey]; ok {
		height, err = strconv.ParseUint(strHeight, 10, 64)
		if err != nil {
			return 0, nil, err
		}
	}
	hexNID := vars[nIDKey]
	nID, err = hex.DecodeString(hexNID)
	if err != nil {
		return 0, nil, err
	}

	return height, nID, nil
}
