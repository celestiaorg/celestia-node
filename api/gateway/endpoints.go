package gateway

import (
	"fmt"
	"net/http"
)

func (h *Handler) RegisterEndpoints(rpc *Server, deprecatedEndpointsEnabled bool) {
	if deprecatedEndpointsEnabled {
		log.Warn("Deprecated endpoints will be removed from the gateway in the next release. Use the RPC instead.")
		// state endpoints
		rpc.RegisterHandlerFunc(balanceEndpoint, h.handleBalanceRequest, http.MethodGet)
		rpc.RegisterHandlerFunc(submitPFBEndpoint, h.handleSubmitPFB, http.MethodPost)

		// staking queries
		rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}", queryDelegationEndpoint, addrKey), h.handleQueryDelegation,
			http.MethodGet)
		rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}", queryUnbondingEndpoint, addrKey), h.handleQueryUnbonding,
			http.MethodGet)
		rpc.RegisterHandlerFunc(queryRedelegationsEndpoint, h.handleQueryRedelegations,
			http.MethodPost)

		// DASer endpoints
		// only register if DASer service is available
		if h.das != nil {
			rpc.RegisterHandlerFunc(dasStateEndpoint, h.handleDASStateRequest, http.MethodGet)
		}
	}

	// state endpoints
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}", balanceEndpoint, addrKey), h.handleBalanceRequest,
		http.MethodGet)
	rpc.RegisterHandlerFunc(submitTxEndpoint, h.handleSubmitTx, http.MethodPost)

	// share endpoints
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}/height/{%s}", namespacedSharesEndpoint, namespaceKey, heightKey),
		h.handleSharesByNamespaceRequest, http.MethodGet)
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}", namespacedSharesEndpoint, namespaceKey),
		h.handleSharesByNamespaceRequest, http.MethodGet)
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}/height/{%s}", namespacedDataEndpoint, namespaceKey, heightKey),
		h.handleDataByNamespaceRequest, http.MethodGet)
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}", namespacedDataEndpoint, namespaceKey),
		h.handleDataByNamespaceRequest, http.MethodGet)

	// DAS endpoints
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}", heightAvailabilityEndpoint, heightKey),
		h.handleHeightAvailabilityRequest, http.MethodGet)

	// header endpoints
	rpc.RegisterHandlerFunc(fmt.Sprintf("%s/{%s}", headerByHeightEndpoint, heightKey), h.handleHeaderRequest,
		http.MethodGet)
	rpc.RegisterHandlerFunc(headEndpoint, h.handleHeadRequest, http.MethodGet)
}
