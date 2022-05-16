package rpc

import (
	"fmt"
	"net/http"
)

func (h *Handler) RegisterEndpoints(rpc *Server) {
	// state endpoints
	rpc.RegisterHandlerFunc(balanceEndpoint, h.handleBalanceRequest, http.MethodGet)
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}", balanceEndpoint, addrKey), h.handleBalanceForAddrRequest,
		http.MethodGet)
	rpc.RegisterHandlerFunc(submitTxEndpoint, h.handleSubmitTx, http.MethodPost)
	rpc.RegisterHandlerFunc(submitPFDEndpoint, h.handleSubmitPFD, http.MethodPost)

	// share endpoints
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}/height/{%s}", namespacedSharesEndpoint, nIDKey, heightKey),
		h.handleSharesByNamespaceRequest, http.MethodGet)
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}", namespacedSharesEndpoint, nIDKey),
		h.handleSharesByNamespaceRequest, http.MethodGet)

	// header endpoints
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}", headerByHeightEndpoint, heightKey), h.handleHeaderRequest,
		http.MethodGet)
	rpc.RegisterHandlerFunc(headEndpoint, h.handleHeadRequest, http.MethodGet)
}
