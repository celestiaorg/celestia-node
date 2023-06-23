package gateway

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/gorilla/mux"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/state"
)

const (
	balanceEndpoint            = "/balance"
	submitTxEndpoint           = "/submit_tx"
	submitPFBEndpoint          = "/submit_pfb"
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
		bal, err = h.state.BalanceForAddress(r.Context(), state.Address{Address: addr})
	} else {
		logDeprecation(balanceEndpoint, "state.Balance")
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
	logDeprecation(submitPFBEndpoint, "blob.Submit or state.SubmitPayForBlob")
	// decode request
	var req submitPFBRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		writeError(w, http.StatusBadRequest, submitPFBEndpoint, err)
		return
	}
	namespace, err := hex.DecodeString(req.NamespaceID)
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

	constructedBlob, err := blob.NewBlobV0(namespace, data)
	if err != nil {
		writeError(w, http.StatusBadRequest, submitPFBEndpoint, err)
		return
	}

	// perform request
	txResp, err := h.state.SubmitPayForBlob(r.Context(), fee, req.GasLimit, []*blob.Blob{constructedBlob})
	if err != nil {
		if txResp == nil {
			// no tx data to return
			writeError(w, http.StatusBadRequest, submitPFBEndpoint, err)
			return
		}
		// if error returned, change status from 200 to 206
		w.WriteHeader(http.StatusPartialContent)
	}

	bs, err := json.Marshal(&txResp)
	if err != nil {
		writeError(w, http.StatusInternalServerError, submitPFBEndpoint, err)
		return
	}

	_, err = w.Write(bs)
	if err != nil {
		log.Errorw("writing response", "endpoint", submitPFBEndpoint, "err", err)
	}
}

func (h *Handler) handleQueryDelegation(w http.ResponseWriter, r *http.Request) {
	logDeprecation(queryDelegationEndpoint, "state.QueryDelegation")
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
	logDeprecation(queryUnbondingEndpoint, "state.QueryUnbonding")
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
	logDeprecation(queryRedelegationsEndpoint, "state.QueryRedelegations")
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

func logDeprecation(endpoint string, alternative string) {
	log.Warn("The " + endpoint + " endpoint is deprecated and will be removed in the next release. Please " +
		"use " + alternative + " from the RPC instead.")
}
