package gateway

import (
	"fmt"
	"net/http"
)

func (h *Handler) RegisterEndpoints(rpc *Server) {
	// state endpoints
	rpc.RegisterHandlerFunc(balanceEndpoint, h.handleBalanceRequest, http.MethodGet)
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}", balanceEndpoint, addrKey), h.handleBalanceRequest,
		http.MethodGet)
	rpc.RegisterHandlerFunc(submitTxEndpoint, h.handleSubmitTx, http.MethodPost)
	rpc.RegisterHandlerFunc(submitPFBEndpoint, h.handleSubmitPFB, http.MethodPost)

	// staking queries
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}", queryDelegationEndpoint, addrKey), h.handleQueryDelegation,
		http.MethodGet)
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}", queryUnbondingEndpoint, addrKey), h.handleQueryUnbonding,
		http.MethodGet)
	rpc.RegisterHandlerFunc(queryRedelegationsEndpoint, h.handleQueryRedelegations,
		http.MethodPost)

	// share endpoints
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}/height/{%s}", namespacedSharesEndpoint, nIDKey, heightKey),
		h.handleSharesByNamespaceRequest, http.MethodGet)
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}", namespacedSharesEndpoint, nIDKey),
		h.handleSharesByNamespaceRequest, http.MethodGet)
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}/height/{%s}", namespacedDataEndpoint, nIDKey, heightKey),
		h.handleDataByNamespaceRequest, http.MethodGet)
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}", namespacedDataEndpoint, nIDKey),
		h.handleDataByNamespaceRequest, http.MethodGet)

	// DAS endpoints
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}", heightAvailabilityEndpoint, heightKey),
		h.handleHeightAvailabilityRequest, http.MethodGet)

	// header endpoints
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}", headerByHeightEndpoint, heightKey), h.handleHeaderRequest,
		http.MethodGet)
	rpc.RegisterHandlerFunc(headEndpoint, h.handleHeadRequest, http.MethodGet)

	// DASer endpoints
	// only register if DASer service is available
	if h.das != nil {
		rpc.RegisterHandlerFunc(dasStateEndpoint, h.handleDASStateRequest, http.MethodGet)
	}
}
