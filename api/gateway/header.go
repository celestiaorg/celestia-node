package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/celestiaorg/celestia-node/header"
	modheader "github.com/celestiaorg/celestia-node/nodebuilder/header"
)

const (
	headEndpoint           = "/head"
	headerByHeightEndpoint = "/header"
)

var (
	heightKey = "height"
)

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

	header, code, err := headerGetByHeight(r.Context(), height, h.header)
	if err != nil {
		writeError(w, code, endpoint, err)
		return nil, err
	}

	return header, nil
}

func headerGetByHeight(ctx context.Context, height int, mod modheader.Module) (*header.ExtendedHeader, int, error) {
	//TODO: change this to NetworkHead once the adjacency in the store is fixed.
	head, err := mod.LocalHead(ctx)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	if localHead := int(head.Height()); localHead+1 < height { // +1 to accommodate for the network and synchronization lag
		err = fmt.Errorf(
			"current head local chain head: %d is lower than requested height: %d"+
				" give header sync some time and retry later", localHead, height)
		return nil, http.StatusServiceUnavailable, err
	}

	header, err := mod.GetByHeight(ctx, uint64(height))
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	return header, 0, nil
}
