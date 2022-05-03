package state

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/gorilla/mux"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/node/rpc"
)

const (
	balanceEndpoint   = "/balance"
	submitTxEndpoint  = "/submit_tx"
	submitPFDEndpoint = "/submit_pfd"
)

var (
	addrKey = "address"
	txKey   = "tx"

	log = logging.Logger("state/rpc")
)

// submitPFDRequest represents a request to submit a PayForData
// transaction.
type submitPFDRequest struct {
	NamespaceID string `json:"namespace_id"`
	Data        string `json:"data"`
	GasLimit    uint64 `json:"gas_limit"`
}

func (s *Service) RegisterEndpoints(rpc *rpc.Server) {
	rpc.RegisterHandlerFunc(balanceEndpoint, s.handleBalanceRequest, http.MethodGet)
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}", balanceEndpoint, addrKey), s.handleBalanceForAddrRequest,
		http.MethodGet)
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}", submitTxEndpoint, txKey), s.handleSubmitTx, http.MethodPost)
	rpc.RegisterHandlerFunc(submitPFDEndpoint, s.handleSubmitPFD, http.MethodPost)
}

func (s *Service) handleBalanceRequest(w http.ResponseWriter, r *http.Request) {
	bal, err := s.accessor.Balance(r.Context())
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

func (s *Service) handleBalanceForAddrRequest(w http.ResponseWriter, r *http.Request) {
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
	bal, err := s.accessor.BalanceForAddress(r.Context(), addr)
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

func (s *Service) handleSubmitTx(w http.ResponseWriter, r *http.Request) {
	// read and parse request
	txStr := mux.Vars(r)[txKey]
	raw, err := hex.DecodeString(txStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Errorw("serving request", "endpoint", submitTxEndpoint, "err", err)
		return
	}
	// perform request
	txResp, err := s.accessor.SubmitTx(r.Context(), raw)
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

func (s *Service) handleSubmitPFD(w http.ResponseWriter, r *http.Request) {
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
	txResp, err := s.accessor.SubmitPayForData(r.Context(), nID, data, req.GasLimit)
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
