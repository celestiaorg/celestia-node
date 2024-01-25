package gateway

import (
	"fmt"
	"net/http"
)

func (h *Handler) RegisterEndpoints(rpc *Server) {
	// state endpoints
	rpc.RegisterHandlerFunc(
		fmt.Sprintf("%s/{%s}", balanceEndpoint, addrKey),
		h.handleBalanceRequest,
		http.MethodGet,
	)

	rpc.RegisterHandlerFunc(
		submitTxEndpoint,
		h.handleSubmitTx,
		http.MethodPost,
	)

	rpc.RegisterHandlerFunc(
		healthEndpoint,
		h.handleHealthRequest,
		http.MethodGet,
	)

	// share endpoints
	rpc.RegisterHandlerFunc(
		fmt.Sprintf(
			"%s/{%s}/height/{%s}",
			namespacedSharesEndpoint,
			namespaceKey,
			heightKey,
		),
		h.handleSharesByNamespaceRequest,
		http.MethodGet,
	)

	rpc.RegisterHandlerFunc(
		fmt.Sprintf("%s/{%s}", namespacedSharesEndpoint, namespaceKey),
		h.handleSharesByNamespaceRequest,
		http.MethodGet,
	)

	rpc.RegisterHandlerFunc(
		fmt.Sprintf("%s/{%s}/height/{%s}", namespacedDataEndpoint, namespaceKey, heightKey),
		h.handleDataByNamespaceRequest,
		http.MethodGet,
	)

	rpc.RegisterHandlerFunc(
		fmt.Sprintf("%s/{%s}", namespacedDataEndpoint, namespaceKey),
		h.handleDataByNamespaceRequest,
		http.MethodGet,
	)

	// DAS endpoints
	rpc.RegisterHandlerFunc(
		fmt.Sprintf("%s/{%s}", heightAvailabilityEndpoint, heightKey),
		h.handleHeightAvailabilityRequest,
		http.MethodGet,
	)

	// header endpoints
	rpc.RegisterHandlerFunc(
		fmt.Sprintf("%s/{%s}", headerByHeightEndpoint, heightKey),
		h.handleHeaderRequest,
		http.MethodGet,
	)

	rpc.RegisterHandlerFunc(headEndpoint, h.handleHeadRequest, http.MethodGet)
}
