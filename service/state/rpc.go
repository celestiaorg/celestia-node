package state

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/gorilla/mux"
	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-node/node/rpc"
)

var log = logging.Logger("state/rpc")

var (
	addrKey = "address"
	txKey   = "tx"
)

func (s *Service) RegisterEndpoints(rpc *rpc.Server) {
	rpc.RegisterHandlerFunc("/balance", s.handleBalanceRequest, http.MethodGet)
	rpc.RegisterHandlerFunc(fmt.Sprintf("/balance/{%s}", addrKey), s.handleBalanceForAddrRequest, http.MethodPost)
	rpc.RegisterHandlerFunc(fmt.Sprintf("/submit_tx/{%s}", txKey), s.handleSubmitTx, http.MethodPost)
}

// TODO @renaynay simplify logic between balance get and balance post
func (s *Service) handleBalanceRequest(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	bal, err := s.accessor.Balance(ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorw("serving /balance request", "err", err)
		return
	}
	resp, err := bal.Marshal()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorw("serving /balance request", "err", err)
		return
	}
	w.WriteHeader(http.StatusOK) // TODO @renaynay set header before or after writing resp?
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("writing /balance response", "err", err)
	}
}

func (s *Service) handleBalanceForAddrRequest(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
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
	bal, err := s.accessor.BalanceForAddress(ctx, addr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorw("serving /balance request", "err", err)
		return
	}
	resp, err := bal.Marshal()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorw("serving /balance request", "err", err)
		return
	}
	w.WriteHeader(http.StatusOK) // TODO @renaynay set header before or after writing resp?
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("writing /balance response", "err", err)
	}
}

func (s *Service) handleSubmitTx(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	// read and parse request
	vars := mux.Vars(r)
	txStr := vars[txKey]
	// perform request
	txResp, err := s.accessor.SubmitTx(ctx, []byte(txStr))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorw("serving /submit_tx request", "err", err)
		return
	}
	resp, err := txResp.Marshal()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorw("serving /submit_tx request", "err", err)
		return
	}
	w.WriteHeader(http.StatusOK) // TODO @renaynay set header before or after writing resp?
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("writing /balance response", "err", err)
	}
}
