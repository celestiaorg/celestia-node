package gateway

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/celestiaorg/celestia-node/state"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/gorilla/mux"
)

const (
	balanceEndpoint            = "/balance"
	submitTxEndpoint           = "/submit_tx"
	submitPFBEndpoint          = "/submit_pfb"
	transferEndpoint           = "/transfer"
	delegationEndpoint         = "/delegate"
	undelegationEndpoint       = "/begin_unbonding"
	cancelUnbondingEndpoint    = "/cancel_unbond"
	beginRedelegationEndpoint  = "/begin_redelegate"
	queryDelegationEndpoint    = "/query_delegation"
	queryUnbondingEndpoint     = "/query_unbonding"
	queryRedelegationsEndpoint = "/query_redelegations"
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

// submitPFBRequest represents a request to submit a PayForBlob
// transaction.
type submitPFBRequest struct {
	NamespaceID string `json:"namespace_id"`
	Data        string `json:"data"`
	Fee         int64  `json:"fee"`
	GasLimit    uint64 `json:"gas_limit"`
}

type transferRequest struct {
	To       string `json:"to"`
	Amount   int64  `json:"amount"`
	Fee      int64  `json:"fee"`
	GasLimit uint64 `json:"gas_limit"`
}

// delegationRequest represents a request for both delegation
// and for beginning and canceling undelegation
type delegationRequest struct {
	To       string `json:"to"`
	Amount   int64  `json:"amount"`
	Fee      int64  `json:"fee"`
	GasLimit uint64 `json:"gas_limit"`
}

// redelegationRequest represents a request for redelegation
type redelegationRequest struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Amount   int64  `json:"amount"`
	Fee      int64  `json:"fee"`
	GasLimit uint64 `json:"gas_limit"`
}

// unbondRequest represents a request to begin unbonding
type unbondRequest struct {
	From     string `json:"from"`
	Amount   int64  `json:"amount"`
	Fee      int64  `json:"fee"`
	GasLimit uint64 `json:"gas_limit"`
}

// cancelUnbondRequest represents a request to cancel unbonding
type cancelUnbondRequest struct {
	From     string `json:"from"`
	Amount   int64  `json:"amount"`
	Height   int64  `json:"height"`
	Fee      int64  `json:"fee"`
	GasLimit uint64 `json:"gas_limit"`
}

// queryRedelegationsRequest represents a request to query redelegations
type queryRedelegationsRequest struct {
	From string `json:"from"`
	To   string `json:"to"`
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

func (h *Handler) handleSubmitPFB(w http.ResponseWriter, r *http.Request) {
	// decode request
	var req submitPFBRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		writeError(w, http.StatusBadRequest, submitPFBEndpoint, err)
		return
	}
	nID, err := hex.DecodeString(req.NamespaceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, submitPFBEndpoint, err)
		return
	}
	data, err := hex.DecodeString(req.Data)
	if err != nil {
		writeError(w, http.StatusBadRequest, submitPFBEndpoint, err)
		return
	}
	fee := types.NewInt(req.Fee)
	// perform request
	txResp, err := h.state.SubmitPayForBlob(r.Context(), nID, data, fee, req.GasLimit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, submitPFBEndpoint, err)
		return
	}
	resp, err := json.Marshal(txResp)
	if err != nil {
		writeError(w, http.StatusInternalServerError, submitPFBEndpoint, err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("writing response", "endpoint", submitPFBEndpoint, "err", err)
	}
}

func (h *Handler) handleTransfer(w http.ResponseWriter, r *http.Request) {
	var req transferRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		writeError(w, http.StatusBadRequest, transferEndpoint, err)
		return
	}
	addr, err := types.AccAddressFromBech32(req.To)
	if err != nil {
		// first check if it is a validator address and can be converted
		valAddr, err := types.ValAddressFromBech32(req.To)
		if err != nil {
			writeError(w, http.StatusBadRequest, transferEndpoint, ErrInvalidAddressFormat)
			return
		}
		addr = valAddr.Bytes()
	}
	amount := types.NewInt(req.Amount)
	fee := types.NewInt(req.Fee)

	txResp, err := h.state.Transfer(r.Context(), addr, amount, fee, req.GasLimit)
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
	addr, err := types.ValAddressFromBech32(req.To)
	if err != nil {
		writeError(w, http.StatusBadRequest, delegationEndpoint, err)
		return
	}
	amount := types.NewInt(req.Amount)
	fee := types.NewInt(req.Fee)

	txResp, err := h.state.Delegate(r.Context(), addr, amount, fee, req.GasLimit)
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
	addr, err := types.ValAddressFromBech32(req.From)
	if err != nil {
		writeError(w, http.StatusBadRequest, undelegationEndpoint, err)
		return
	}
	amount := types.NewInt(req.Amount)
	fee := types.NewInt(req.Fee)

	txResp, err := h.state.Undelegate(r.Context(), addr, amount, fee, req.GasLimit)
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
	addr, err := types.ValAddressFromBech32(req.From)
	if err != nil {
		writeError(w, http.StatusBadRequest, cancelUnbondingEndpoint, err)
		return
	}
	amount := types.NewInt(req.Amount)
	height := types.NewInt(req.Height)
	fee := types.NewInt(req.Fee)
	txResp, err := h.state.CancelUnbondingDelegation(r.Context(), addr, amount, height, fee, req.GasLimit)
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
	fee := types.NewInt(req.Fee)

	txResp, err := h.state.BeginRedelegate(r.Context(), srcAddr, dstAddr, amount, fee, req.GasLimit)
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

func (h *Handler) handleQueryDelegation(w http.ResponseWriter, r *http.Request) {
	// read and parse request
	vars := mux.Vars(r)
	addrStr, exists := vars[addrKey]
	if !exists {
		writeError(w, http.StatusBadRequest, queryDelegationEndpoint, ErrMissingAddress)
		return
	}

	// convert address to Address type
	addr, err := types.ValAddressFromBech32(addrStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, queryDelegationEndpoint, err)
		return
	}
	delegation, err := h.state.QueryDelegation(r.Context(), addr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, queryDelegationEndpoint, err)
		return
	}
	resp, err := json.Marshal(delegation)
	if err != nil {
		writeError(w, http.StatusInternalServerError, queryDelegationEndpoint, err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("writing response", "endpoint", queryDelegationEndpoint, "err", err)
	}
}

func (h *Handler) handleQueryUnbonding(w http.ResponseWriter, r *http.Request) {
	// read and parse request
	vars := mux.Vars(r)
	addrStr, exists := vars[addrKey]
	if !exists {
		writeError(w, http.StatusBadRequest, queryUnbondingEndpoint, ErrMissingAddress)
		return
	}

	// convert address to Address type
	addr, err := types.ValAddressFromBech32(addrStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, queryUnbondingEndpoint, err)
		return
	}
	unbonding, err := h.state.QueryUnbonding(r.Context(), addr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, queryUnbondingEndpoint, err)
		return
	}
	resp, err := json.Marshal(unbonding)
	if err != nil {
		writeError(w, http.StatusInternalServerError, queryUnbondingEndpoint, err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("writing response", "endpoint", queryUnbondingEndpoint, "err", err)
	}
}

func (h *Handler) handleQueryRedelegations(w http.ResponseWriter, r *http.Request) {
	var req queryRedelegationsRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		writeError(w, http.StatusBadRequest, queryRedelegationsEndpoint, err)
		return
	}
	srcValAddr, err := types.ValAddressFromBech32(req.From)
	if err != nil {
		writeError(w, http.StatusBadRequest, queryRedelegationsEndpoint, err)
		return
	}
	dstValAddr, err := types.ValAddressFromBech32(req.To)
	if err != nil {
		writeError(w, http.StatusBadRequest, queryRedelegationsEndpoint, err)
		return
	}
	unbonding, err := h.state.QueryRedelegations(r.Context(), srcValAddr, dstValAddr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, queryRedelegationsEndpoint, err)
		return
	}
	resp, err := json.Marshal(unbonding)
	if err != nil {
		writeError(w, http.StatusInternalServerError, queryRedelegationsEndpoint, err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("writing response", "endpoint", queryRedelegationsEndpoint, "err", err)
	}
}
