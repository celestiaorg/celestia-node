package gateway

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func TestRegisterEndpoints(t *testing.T) {
	handler := &Handler{}
	rpc := NewServer("localhost", "6969")

	handler.RegisterEndpoints(rpc)

	testCases := []struct {
		name     string
		path     string
		method   string
		expected bool
	}{
		{
			name:     "Get balance endpoint",
			path:     fmt.Sprintf("%s/{%s}", balanceEndpoint, addrKey),
			method:   http.MethodGet,
			expected: true,
		},
		{
			name:     "Submit transaction endpoint",
			path:     submitTxEndpoint,
			method:   http.MethodPost,
			expected: true,
		},
		{
			name:     "Get namespaced shares by height endpoint",
			path:     fmt.Sprintf("%s/{%s}/height/{%s}", namespacedSharesEndpoint, namespaceKey, heightKey),
			method:   http.MethodGet,
			expected: true,
		},
		{
			name:     "Get namespaced shares endpoint",
			path:     fmt.Sprintf("%s/{%s}", namespacedSharesEndpoint, namespaceKey),
			method:   http.MethodGet,
			expected: true,
		},
		{
			name:     "Get namespaced data by height endpoint",
			path:     fmt.Sprintf("%s/{%s}/height/{%s}", namespacedDataEndpoint, namespaceKey, heightKey),
			method:   http.MethodGet,
			expected: true,
		},
		{
			name:     "Get namespaced data endpoint",
			path:     fmt.Sprintf("%s/{%s}", namespacedDataEndpoint, namespaceKey),
			method:   http.MethodGet,
			expected: true,
		},
		{
			name:     "Get health endpoint",
			path:     "/status/health",
			method:   http.MethodGet,
			expected: true,
		},

		// Going forward, we can add previously deprecated and since
		// removed endpoints here to ensure we don't accidentally re-enable
		// them in the future and accidentally expand surface area
		{
			name:     "example totally bogus endpoint",
			path:     fmt.Sprintf("/wutang/{%s}/%s", "chambers", "36"),
			method:   http.MethodGet,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(
				t,
				tc.expected,
				hasEndpointRegistered(rpc.Router(), tc.path, tc.method),
				"Endpoint registration mismatch for: %s %s %s", tc.name, tc.method, tc.path)
		})
	}
}

func hasEndpointRegistered(router *mux.Router, path, method string) bool {
	var registered bool
	err := router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		template, err := route.GetPathTemplate()
		if err != nil {
			return err
		}

		if template == path {
			methods, err := route.GetMethods()
			if err != nil {
				return err
			}

			for _, m := range methods {
				if m == method {
					registered = true
					return nil
				}
			}
		}
		return nil
	})
	if err != nil {
		fmt.Println("Error walking through routes:", err)
		return false
	}

	return registered
}
