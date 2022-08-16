package rpc

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/gorilla/mux"

	"github.com/celestiaorg/celestia-node/service/state"
)

const (
	balanceEndpoint           = "/balance"
	submitTxEndpoint          = "/submit_tx"
	submitPFDEndpoint         = "/submit_pfd"
	transferEndpoint          = "/transfer"
	delegationEndpoint        = "/delegate"
	undelegationEndpoint      = "/begin_unbonding"
	cancelUnbondingEndpoint   = "/cancel_unbond"
	beginRedelegationEndpoint = "/begin_redelegate"
)

var addrKey = "address"

// submitTxRequest represents a request to submit a raw transaction
type submitTxRequest struct {
	Tx string `json:"tx"`
}

// submitPFDRequest represents a request to submit a PayForData
// transaction.
type submitPFDRequest struct {
	NamespaceID string `json:"namespace_id"`
	Data        string `json:"data"`
	GasLimit    uint64 `json:"gas_limit"`
}

type transferRequest struct {
	To       string `json:"to"`
	Amount   int64  `json:"amount"`
	GasLimit uint64 `json:"gas_limit"`
}

// delegationRequest represents a request for both delegation
// and for beginning and canceling undelegation
type delegationRequest struct {
	To       string `json:"to"`
	Amount   int64  `json:"amount"`
	GasLimit uint64 `json:"gas_limit"`
}

// redelegationRequest represents a request for redelegation
type redelegationRequest struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Amount   int64  `json:"amount"`
	GasLimit uint64 `json:"gas_limit"`
}

// unbondRequest represents a request to begin unbonding
type unbondRequest struct {
	From     string `json:"from"`
	Amount   int64  `json:"amount"`
	GasLimit uint64 `json:"gas_limit"`
}

// cancelUnbondRequest represents a request to cancel unbonding
type cancelUnbondRequest struct {
	From     string `json:"from"`
	Amount   int64  `json:"amount"`
	Height   int64  `json:"height"`
	GasLimit uint64 `json:"gas_limit"`
}

func (h *Handler) handleBalanceRequest(w http.ResponseWriter, r *http.Request) {
	var (
		bal *state.Balance
		err error
	)
	// read and parse request
	vars := mux.Vars(r)
	addrStr, exists := vars[addrKey]
	if exists {
		// convert address to Address type
		addr, addrerr := types.AccAddressFromBech32(addrStr)
		if addrerr != nil {
			writeError(w, http.StatusBadRequest, balanceEndpoint, addrerr)
			return
		}
		bal, err = h.state.BalanceForAddress(r.Context(), addr)
	} else {
		bal, err = h.state.Balance(r.Context())
	}
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

func (h *Handler) handleSubmitPFD(w http.ResponseWriter, r *http.Request) {
	// decode request
	var req submitPFDRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		writeError(w, http.StatusBadRequest, submitPFDEndpoint, err)
		return
	}
	nID, err := hex.DecodeString(req.NamespaceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, submitPFDEndpoint, err)
		return
	}
	data, err := hex.DecodeString(req.Data)
	if err != nil {
		writeError(w, http.StatusBadRequest, submitPFDEndpoint, err)
		return
	}
	// perform request
	txResp, err := h.state.SubmitPayForData(r.Context(), nID, data, req.GasLimit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, submitPFDEndpoint, err)
		return
	}
	resp, err := json.Marshal(txResp)
	if err != nil {
		writeError(w, http.StatusInternalServerError, submitPFDEndpoint, err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("writing response", "endpoint", submitPFDEndpoint, "err", err)
	}
}

func (h *Handler) handleTransfer(w http.ResponseWriter, r *http.Request) {
	var req transferRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		writeError(w, http.StatusBadRequest, transferEndpoint, err)
		return
	}
	if req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, transferEndpoint, errors.New("amount must be greater than 0"))
		return
	}
	addr, err := types.AccAddressFromBech32(req.To)
	if err != nil {
		writeError(w, http.StatusBadRequest, transferEndpoint, err)
		return
	}
	amount := types.NewInt(req.Amount)

	txResp, err := h.state.Transfer(r.Context(), addr, amount, req.GasLimit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, transferEndpoint, err)
		return
	}
	resp, err := json.Marshal(txResp)
	if err != nil {
		writeError(w, http.StatusInternalServerError, transferEndpoint, err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("writing response", "endpoint", transferEndpoint, "err", err)
	}
}

func (h *Handler) handleDelegation(w http.ResponseWriter, r *http.Request) {
	var req delegationRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		writeError(w, http.StatusBadRequest, delegationEndpoint, err)
		return
	}
	if req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, delegationEndpoint, errors.New("amount must be greater than 0"))
		return
	}
	addr, err := types.ValAddressFromBech32(req.To)
	if err != nil {
		writeError(w, http.StatusBadRequest, delegationEndpoint, err)
		return
	}
	amount := types.NewInt(req.Amount)

	txResp, err := h.state.Delegate(r.Context(), addr, amount, req.GasLimit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, delegationEndpoint, err)
		return
	}
	resp, err := json.Marshal(txResp)
	if err != nil {
		writeError(w, http.StatusInternalServerError, delegationEndpoint, err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("writing response", "endpoint", delegationEndpoint, "err", err)
	}
}

func (h *Handler) handleUndelegation(w http.ResponseWriter, r *http.Request) {
	var req unbondRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		writeError(w, http.StatusBadRequest, undelegationEndpoint, err)
		return
	}
	if req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, undelegationEndpoint, errors.New("amount must be greater than 0"))
		return
	}
	addr, err := types.ValAddressFromBech32(req.From)
	if err != nil {
		writeError(w, http.StatusBadRequest, undelegationEndpoint, err)
		return
	}
	amount := types.NewInt(req.Amount)

	txResp, err := h.state.Undelegate(r.Context(), addr, amount, req.GasLimit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, undelegationEndpoint, err)
		return
	}
	resp, err := json.Marshal(txResp)
	if err != nil {
		writeError(w, http.StatusInternalServerError, undelegationEndpoint, err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("writing response", "endpoint", undelegationEndpoint, "err", err)
	}
}

func (h *Handler) handleCancelUnbonding(w http.ResponseWriter, r *http.Request) {
	var req cancelUnbondRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		writeError(w, http.StatusBadRequest, cancelUnbondingEndpoint, err)
		return
	}
	if req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, cancelUnbondingEndpoint, errors.New("amount must be greater than 0"))
		return
	}
	addr, err := types.ValAddressFromBech32(req.From)
	if err != nil {
		writeError(w, http.StatusBadRequest, cancelUnbondingEndpoint, err)
		return
	}
	amount := types.NewInt(req.Amount)
	height := types.NewInt(req.Height)
	txResp, err := h.state.CancelUnbondingDelegation(r.Context(), addr, amount, height, req.GasLimit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, cancelUnbondingEndpoint, err)
		return
	}
	resp, err := json.Marshal(txResp)
	if err != nil {
		writeError(w, http.StatusInternalServerError, cancelUnbondingEndpoint, err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("writing response", "endpoint", cancelUnbondingEndpoint, "err", err)
	}
}

func (h *Handler) handleRedelegation(w http.ResponseWriter, r *http.Request) {
	var req redelegationRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		writeError(w, http.StatusBadRequest, beginRedelegationEndpoint, err)
		return
	}
	if req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, beginRedelegationEndpoint, errors.New("amount must be greater than 0"))
		return
	}
	srcAddr, err := types.ValAddressFromBech32(req.From)
	if err != nil {
		writeError(w, http.StatusBadRequest, beginRedelegationEndpoint, err)
		return
	}
	dstAddr, err := types.ValAddressFromBech32(req.To)
	if err != nil {
		writeError(w, http.StatusBadRequest, beginRedelegationEndpoint, err)
		return
	}
	amount := types.NewInt(req.Amount)

	txResp, err := h.state.BeginRedelegate(r.Context(), srcAddr, dstAddr, amount, req.GasLimit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, beginRedelegationEndpoint, err)
		return
	}
	resp, err := json.Marshal(txResp)
	if err != nil {
		writeError(w, http.StatusInternalServerError, beginRedelegationEndpoint, err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("writing response", "endpoint", beginRedelegationEndpoint, "err", err)
	}
}
