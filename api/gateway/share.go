package gateway

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	libshare "github.com/celestiaorg/go-square/v2/share"
)

const (
	namespacedSharesEndpoint = "/namespaced_shares"
	namespacedDataEndpoint   = "/namespaced_data"
)

var namespaceKey = "nid"

// NamespacedSharesResponse represents the response to a
// SharesByNamespace request.
type NamespacedSharesResponse struct {
	Shares []libshare.Share `json:"shares"`
	Height uint64           `json:"height"`
}

// NamespacedDataResponse represents the response to a
// DataByNamespace request.
type NamespacedDataResponse struct {
	Data   [][]byte `json:"data"`
	Height uint64   `json:"height"`
}

func (h *Handler) handleSharesByNamespaceRequest(w http.ResponseWriter, r *http.Request) {
	height, namespace, err := parseGetByNamespaceArgs(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, namespacedSharesEndpoint, err)
		return
	}
	shares, err := h.getShares(r.Context(), height, namespace)
	if err != nil {
		writeError(w, http.StatusInternalServerError, namespacedSharesEndpoint, err)
		return
	}
	resp, err := json.Marshal(&NamespacedSharesResponse{
		Shares: shares,
		Height: height,
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
	height, namespace, err := parseGetByNamespaceArgs(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, namespacedDataEndpoint, err)
		return
	}
	shares, err := h.getShares(r.Context(), height, namespace)
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
		Height: height,
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

func (h *Handler) getShares(
	ctx context.Context,
	height uint64,
	namespace libshare.Namespace,
) ([]libshare.Share, error) {
	shares, err := h.share.GetSharesByNamespace(ctx, height, namespace)
	if err != nil {
		return nil, err
	}

	return shares.Flatten(), nil
}

func dataFromShares(input []libshare.Share) (data [][]byte, err error) {
	sequences, err := libshare.ParseShares(input, false)
	if err != nil {
		return nil, err
	}
	for _, sequence := range sequences {
		raw, err := sequence.RawData()
		if err != nil {
			return nil, err
		}
		data = append(data, raw)
	}
	return data, nil
}

func parseGetByNamespaceArgs(r *http.Request) (height uint64, namespace libshare.Namespace, err error) {
	vars := mux.Vars(r)
	// if a height was given, parse it, otherwise get namespaced shares/data from the latest header
	if strHeight, ok := vars[heightKey]; ok {
		height, err = strconv.ParseUint(strHeight, 10, 64)
		if err != nil {
			return 0, libshare.Namespace{}, err
		}
	}
	hexNamespace := vars[namespaceKey]
	nsString, err := hex.DecodeString(hexNamespace)
	if err != nil {
		return 0, libshare.Namespace{}, err
	}
	ns, err := libshare.NewNamespaceFromBytes(nsString)
	if err != nil {
		return 0, libshare.Namespace{}, err
	}
	namespace = ns
	return height, namespace, namespace.ValidateForData()
}
