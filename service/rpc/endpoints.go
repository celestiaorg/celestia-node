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
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}", submitTxEndpoint, txKey), h.handleSubmitTx, http.MethodPost)
	rpc.RegisterHandlerFunc(submitPFDEndpoint, h.handleSubmitPFD, http.MethodPost)

	// share endpoints
	rpc.RegisterHandlerFunc(namespacedSharesEndpoint, h.handleSharesByNamespaceRequest, http.MethodGet)

	// header endpoints
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}", headerByHeightEndpoint, heightKey), h.handleHeaderRequest,
		http.MethodGet)
}
