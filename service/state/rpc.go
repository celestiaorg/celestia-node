package state

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/gorilla/mux"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/node/rpc"
)

var (
	addrKey = "address"
	txKey   = "tx"

	log = logging.Logger("state/rpc")
)

type submitPFDRequest struct {
	NamespaceID string `json:"namespace_id"`
	Data        string `json:"data"`
	GasLimit    uint64 `json:"gas_limit"`
}

func (s *Service) RegisterEndpoints(rpc *rpc.Server) {
	rpc.RegisterHandlerFunc("/balance", s.handleBalanceRequest, http.MethodGet)
	rpc.RegisterHandlerFunc(fmt.Sprintf("/balance/{%s}", addrKey), s.handleBalanceForAddrRequest, http.MethodGet)
	rpc.RegisterHandlerFunc(fmt.Sprintf("/submit_tx/{%s}", txKey), s.handleSubmitTx, http.MethodPost)
	rpc.RegisterHandlerFunc("/submit_pfd", s.handleSubmitPFD, http.MethodPost)
}

func (s *Service) handleBalanceRequest(w http.ResponseWriter, r *http.Request) {
	bal, err := s.accessor.Balance(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorw("serving /balance request", "err", err)
		return
	}
	resp, err := json.Marshal(bal)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorw("serving /balance request", "err", err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("writing /balance response", "err", err)
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
		log.Errorw("getting AccAddressFromBech32", "err", err)
		return
	}
	bal, err := s.accessor.BalanceForAddress(r.Context(), addr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorw("serving /balance request", "err", err)
		return
	}
	resp, err := json.Marshal(bal)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorw("serving /balance request", "err", err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("writing /balance response", "err", err)
	}
}

func (s *Service) handleSubmitTx(w http.ResponseWriter, r *http.Request) {
	// read and parse request
	txStr := mux.Vars(r)[txKey]
	raw, err := hex.DecodeString(txStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Errorw("serving /submit_tx request", "err", err)
		return
	}
	// perform request
	txResp, err := s.accessor.SubmitTx(r.Context(), raw)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorw("serving /submit_tx request", "err", err)
		return
	}
	resp, err := json.Marshal(txResp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorw("serving /submit_tx request", "err", err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("writing /balance response", "err", err)
	}
}

func (s *Service) handleSubmitPFD(w http.ResponseWriter, r *http.Request) {
	// parse body from request
	raw, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Errorw("serving /submit_pfd request", "err", err)
		return
	}
	// decode request
	req := new(submitPFDRequest)
	err = json.Unmarshal(raw, req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Errorw("serving /submit_pfd request", "err", err)
		return
	}

	nID, err := hex.DecodeString(req.NamespaceID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Errorw("serving /submit_pfd request", "err", err)
		return
	}
	data, err := hex.DecodeString(req.Data)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Errorw("serving /submit_pfd request", "err", err)
		return
	}

	// perform request
	txResp, err := s.SubmitPayForData(r.Context(), nID, data, req.GasLimit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, werr := w.Write([]byte(err.Error()))
		if werr != nil {
			log.Errorw("writing /submit_pfd response", "err", werr)
		}
		log.Errorw("serving /submit_pfd request", "err", err)
		return
	}
	resp, err := json.Marshal(txResp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorw("serving /submit_pfd request", "err", err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("writing /submit_pfd response", "err", err)
	}
}
