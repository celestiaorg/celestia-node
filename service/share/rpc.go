package share

import (
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/celestiaorg/celestia-node/node/rpc"
)

const namespacedSharesEndpoint = "/namespaced_shares"

// sharesByNamespaceRequest represents a `GetSharesByNamespace`
// request payload
type sharesByNamespaceRequest struct {
	NamespaceID string `json:"namespace_id"`
	Root        string `json:"root"`
}

func (s *service) RegisterEndpoints(rpc *rpc.Server) {
	rpc.RegisterHandlerFunc(namespacedSharesEndpoint, s.handleSharesByNamespaceRequest, http.MethodGet)
}

func (s *service) handleSharesByNamespaceRequest(w http.ResponseWriter, r *http.Request) {
	// unmarshal payload
	var req sharesByNamespaceRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Errorw("serving request", "endpoint", namespacedSharesEndpoint, "err", err)
		return
	}
	// decode namespaceID
	nID, err := hex.DecodeString(req.NamespaceID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Errorw("serving request", "endpoint", namespacedSharesEndpoint, "err", err)
		return
	}
	// decode and unmarshal root
	rawRoot, err := hex.DecodeString(req.Root)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Errorw("serving request", "endpoint", namespacedSharesEndpoint, "err", err)
		return
	}
	var root Root
	err = json.Unmarshal(rawRoot, &root)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Errorw("serving request", "endpoint", namespacedSharesEndpoint, "err", err)
		return
	}
	// perform request
	shares, err := s.GetSharesByNamespace(r.Context(), &root, nID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, werr := w.Write([]byte(err.Error()))
		if werr != nil {
			log.Errorw("writing response", "endpoint", namespacedSharesEndpoint, "err", werr)
		}
		log.Errorw("serving request", "endpoint", namespacedSharesEndpoint, "err", err)
		return
	}
	resp, err := json.Marshal(shares)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorw("serving request", "endpoint", namespacedSharesEndpoint, "err", err)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		log.Errorw("serving request", "endpoint", namespacedSharesEndpoint, "err", err)
	}
}
