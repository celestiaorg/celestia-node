package gateway

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/gorilla/mux"

	"github.com/celestiaorg/celestia-node/state"
)

const (
	balanceEndpoint  = "/balance"
	submitTxEndpoint = "/submit_tx"
)

const addrKey = "address"

var (
	ErrInvalidAddressFormat = errors.New("address must be a valid account or validator address")
	ErrMissingAddress       = errors.New("address not specified")
)

// submitTxRequest represents a request to submit a raw transaction
type submitTxRequest struct {
	Tx string `json:"tx"`
}

func (h *Handler) handleBalanceRequest(w http.ResponseWriter, r *http.Request) {
	var (
		bal *state.Balance
		err error
	)
	// read and parse request
	vars := mux.Vars(r)
	addrStr, exists := vars[addrKey]
	if !exists {
		writeError(w, http.StatusBadRequest, balanceEndpoint, errors.New("invalid endpoint"))
		return
	}

	// convert address to Address type
	var addr state.AccAddress
	addr, err = types.AccAddressFromBech32(addrStr)
	if err != nil {
		// first check if it is a validator address and can be converted
		valAddr, err := types.ValAddressFromBech32(addrStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, balanceEndpoint, ErrInvalidAddressFormat)
			return
		}
		addr = valAddr.Bytes()
	}

	bal, err = h.state.BalanceForAddress(r.Context(), state.Address{Address: addr})
	if err != nil {
		writeError(w, http.StatusInternalServerError, balanceEndpoint, err)
		return
	}
	resp, err := json.Marshal(bal)
	if err != nil {
		writeError(w, http.StatusInternalServerError, balanceEndpoint, err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("writing response", "endpoint", balanceEndpoint, "err", err)
	}
}

func (h *Handler) handleSubmitTx(w http.ResponseWriter, r *http.Request) {
	// decode request
	var req submitTxRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		writeError(w, http.StatusBadRequest, submitTxEndpoint, err)
		return
	}
	rawTx, err := hex.DecodeString(req.Tx)
	if err != nil {
		writeError(w, http.StatusBadRequest, submitTxEndpoint, err)
		return
	}
	// perform request
	txResp, err := h.state.SubmitTx(r.Context(), rawTx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, submitTxEndpoint, err)
		return
	}
	resp, err := json.Marshal(txResp)
	if err != nil {
		writeError(w, http.StatusInternalServerError, submitTxEndpoint, err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("writing response", "endpoint", submitTxEndpoint, "err", err)
	}
}
