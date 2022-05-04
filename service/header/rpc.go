package header

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/celestiaorg/celestia-node/node/rpc"
)

const (
	headerByHeightEndpoint = "/header"
	dahByHeightEndpoint    = "/dah"
)

var (
	rpcHeightKey = "height"
)

type dahResponse struct {
	DAH string `json:"dah"`
}

func (s *Service) RegisterEndpoints(rpc *rpc.Server) {
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}", headerByHeightEndpoint, rpcHeightKey), s.handleHeaderRequest,
		http.MethodGet)
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}", dahByHeightEndpoint, rpcHeightKey), s.handleRootRequest,
		http.MethodGet)
}

func (s *Service) handleHeaderRequest(w http.ResponseWriter, r *http.Request) {
	header, err := s.performGetHeaderRequest(w, r, headerByHeightEndpoint)
	if err != nil {
		// return here as we've already logged and written the error
		return
	}
	// marshal and write response
	resp, err := json.Marshal(header)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorw("serving request", "endpoint", headerByHeightEndpoint, "err", err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("writing response", "endpoint", headerByHeightEndpoint, "err", err)
		return
	}
}

func (s *Service) handleRootRequest(w http.ResponseWriter, r *http.Request) {
	header, err := s.performGetHeaderRequest(w, r, dahByHeightEndpoint)
	if err != nil {
		// return here as we've already logged and written the error
		return
	}
	// marshal DAH from header and write it
	resp, err := json.Marshal(header.DAH)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorw("serving request", "endpoint", dahByHeightEndpoint, "err", err)
		return
	}
	// write response
	dahResp, err := json.Marshal(&dahResponse{
		DAH: hex.EncodeToString(resp),
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorw("serving request", "endpoint", dahByHeightEndpoint, "err", err)
		return
	}
	_, err = w.Write(dahResp)
	if err != nil {
		log.Errorw("writing response", "endpoint", dahByHeightEndpoint, "err", err)
		return
	}
}

func (s *Service) performGetHeaderRequest(
	w http.ResponseWriter,
	r *http.Request,
	endpoint string,
) (*ExtendedHeader, error) {
	// read and parse request
	vars := mux.Vars(r)
	heightStr := vars[rpcHeightKey]
	height, err := strconv.Atoi(heightStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, werr := w.Write([]byte("must provide height as int"))
		if werr != nil {
			log.Errorw("writing response", "endpoint", endpoint, "err", werr)
		}
		log.Errorw("serving request", "endpoint", endpoint, "err", err)
		return nil, err
	}
	// perform request
	header, err := s.GetByHeight(r.Context(), uint64(height))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, werr := w.Write([]byte(err.Error()))
		if werr != nil {
			log.Errorw("writing response", "endpoint", endpoint, "err", werr)
		}
		log.Errorw("serving request", "endpoint", endpoint, "err", err)
		return nil, err
	}
	return header, nil
}
