package rpc

import (
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/gorilla/mux"
)

const (
	balanceEndpoint   = "/balance"
	submitTxEndpoint  = "/submit_tx"
	submitPFDEndpoint = "/submit_pfd"
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

func (h *Handler) handleBalanceRequest(w http.ResponseWriter, r *http.Request) {
	bal, err := h.state.Balance(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, werr := w.Write([]byte(err.Error()))
		if werr != nil {
			log.Errorw("writing response", "endpoint", balanceEndpoint, "err", werr)
		}
		log.Errorw("serving request", "endpoint", balanceEndpoint, "err", err)
		return
	}
	resp, err := json.Marshal(bal)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorw("serving request", "endpoint", balanceEndpoint, "err", err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("writing response", "endpoint", balanceEndpoint, "err", err)
	}
}

func (h *Handler) handleBalanceForAddrRequest(w http.ResponseWriter, r *http.Request) {
	// read and parse request
	vars := mux.Vars(r)
	addrStr := vars[addrKey]
	// convert address to Address type
	addr, err := types.AccAddressFromBech32(addrStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, werr := w.Write([]byte(err.Error()))
		if werr != nil {
			log.Errorw("writing response", "endpoint", balanceEndpoint, "err", werr)
		}
		log.Errorw("serving request", "endpoint", balanceEndpoint, "err", err)
		return
	}
	bal, err := h.state.BalanceForAddress(r.Context(), addr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, werr := w.Write([]byte(err.Error()))
		if werr != nil {
			log.Errorw("writing response", "endpoint", balanceEndpoint, "err", werr)
		}
		log.Errorw("serving request", "endpoint", balanceEndpoint, "err", err)
		return
	}
	resp, err := json.Marshal(bal)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorw("serving request", "endpoint", balanceEndpoint, "err", err)
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
		w.WriteHeader(http.StatusBadRequest)
		log.Errorw("serving request", "endpoint", submitTxEndpoint, "err", err)
		return
	}
	rawTx, err := hex.DecodeString(req.Tx)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Errorw("serving request", "endpoint", submitTxEndpoint, "err", err)
		return
	}
	// perform request
	txResp, err := h.state.SubmitTx(r.Context(), rawTx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, werr := w.Write([]byte(err.Error()))
		if werr != nil {
			log.Errorw("writing response", "endpoint", submitTxEndpoint, "err", werr)
		}
		log.Errorw("serving request", "endpoint", submitTxEndpoint, "err", err)
		return
	}
	resp, err := json.Marshal(txResp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorw("serving request", "endpoint", submitTxEndpoint, "err", err)
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
		w.WriteHeader(http.StatusBadRequest)
		log.Errorw("serving request", "endpoint", submitPFDEndpoint, "err", err)
		return
	}
	nID, err := hex.DecodeString(req.NamespaceID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Errorw("serving request", "endpoint", submitPFDEndpoint, "err", err)
		return
	}
	data, err := hex.DecodeString(req.Data)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Errorw("serving request", "endpoint", submitPFDEndpoint, "err", err)
		return
	}
	// perform request
	txResp, err := h.state.SubmitPayForData(r.Context(), nID, data, req.GasLimit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, werr := w.Write([]byte(err.Error()))
		if werr != nil {
			log.Errorw("writing response", "endpoint", submitPFDEndpoint, "err", werr)
		}
		log.Errorw("serving request", "endpoint", submitPFDEndpoint, "err", err)
		return
	}
	resp, err := json.Marshal(txResp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorw("serving request", "endpoint", submitPFDEndpoint, "err", err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("writing response", "endpoint", submitPFDEndpoint, "err", err)
	}
}
