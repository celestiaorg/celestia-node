package gateway

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/celestiaorg/celestia-node/header"
)

const (
	headEndpoint           = "/head"
	headerByHeightEndpoint = "/header"
)

var heightKey = "height"

func (h *Handler) handleHeadRequest(w http.ResponseWriter, r *http.Request) {
	head, err := h.header.LocalHead(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, headEndpoint, err)
		return
	}
	resp, err := json.Marshal(head)
	if err != nil {
		writeError(w, http.StatusInternalServerError, headEndpoint, err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("writing response", "endpoint", headEndpoint, "err", err)
		return
	}
}

func (h *Handler) handleHeaderRequest(w http.ResponseWriter, r *http.Request) {
	header, err := h.performGetHeaderRequest(w, r, headerByHeightEndpoint)
	if err != nil {
		// return here as we've already logged and written the error
		return
	}
	// marshal and write response
	resp, err := json.Marshal(header)
	if err != nil {
		writeError(w, http.StatusInternalServerError, headerByHeightEndpoint, err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("writing response", "endpoint", headerByHeightEndpoint, "err", err)
		return
	}
}

func (h *Handler) performGetHeaderRequest(
	w http.ResponseWriter,
	r *http.Request,
	endpoint string,
) (*header.ExtendedHeader, error) {
	// read and parse request
	vars := mux.Vars(r)
	heightStr := vars[heightKey]
	height, err := strconv.Atoi(heightStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, endpoint, err)
		return nil, err
	}

	header, err := h.header.GetByHeight(r.Context(), uint64(height))
	if err != nil {
		writeError(w, http.StatusInternalServerError, endpoint, err)
		return nil, err
	}

	return header, nil
}
