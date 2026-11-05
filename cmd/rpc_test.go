package cmd

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristalhq/jwt/v5"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/api/rpc/perms"
	"github.com/celestiaorg/celestia-node/libs/authtoken"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func TestInitClientAuthentication(t *testing.T) {
	tests := []struct {
		name     string
		url      bool
		store    bool
		skipAuth bool
		token    string
		deny     bool
	}{
		{name: "remote without token", url: true},
		{name: "remote with token", url: true, token: "provided"},
		{name: "remote requiring authentication rejects missing token", url: true, deny: true},
		{name: "remote with store", url: true, store: true},
		{name: "remote with token and store", url: true, store: true, token: "provided"},
		{name: "remote with unauthenticated store", url: true, store: true, skipAuth: true},
		{name: "local with store", store: true},
		{name: "local with unauthenticated store", store: true, skipAuth: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := make(chan string, 1)
			api := &node.API{}
			api.Internal.Ready = func(context.Context) (bool, error) { return true, nil }
			rpcServer := jsonrpc.NewServer()
			rpcServer.Register("node", api)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				headers <- r.Header.Get("Authorization")
				if tt.deny {
					http.Error(w, "authentication required", http.StatusUnauthorized)
					return
				}
				rpcServer.ServeHTTP(w, r)
			}))
			t.Cleanup(server.Close)

			command := &cobra.Command{}
			command.Flags().AddFlagSet(RPCFlags())
			if tt.url {
				require.NoError(t, command.Flags().Set("url", server.URL))
			}
			if tt.token != "" {
				require.NoError(t, command.Flags().Set("token", tt.token))
			}
			var verifier jwt.Verifier
			if tt.store {
				store := t.TempDir()
				cfg := nodebuilder.DefaultConfig(node.Light)
				var err error
				cfg.RPC.Address, cfg.RPC.Port, err = net.SplitHostPort(server.Listener.Addr().String())
				require.NoError(t, err)
				cfg.RPC.SkipAuth = tt.skipAuth
				require.NoError(t, nodebuilder.SaveConfig(filepath.Join(store, "config.toml"), cfg))
				if !tt.skipAuth {
					ks, err := newKeystore(store)
					require.NoError(t, err)
					key, err := generateNewKey(ks)
					require.NoError(t, err)
					verifier, err = jwt.NewVerifierHS(jwt.HS256, key.Body)
					require.NoError(t, err)
				}
				require.NoError(t, command.Flags().Set(nodeStoreFlag, store))
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			command.SetContext(ctx)
			require.NoError(t, InitClient(command, nil))
			client, err := ParseClientFromCtx(command.Context())
			require.NoError(t, err)
			defer client.Close()
			ready, err := client.Node.Ready(ctx)
			if tt.deny {
				require.ErrorContains(t, err, "401")
				require.False(t, ready)
			} else {
				require.NoError(t, err)
				require.True(t, ready)
			}

			header := <-headers
			switch {
			case tt.token != "":
				require.Equal(t, "Bearer "+tt.token, header)
			case tt.store && !tt.skipAuth:
				require.True(t, strings.HasPrefix(header, "Bearer "))
				permissions, err := authtoken.ExtractSignedPermissions(verifier, strings.TrimPrefix(header, "Bearer "))
				require.NoError(t, err)
				require.Equal(t, perms.AllPerms, permissions)
			default:
				require.Empty(t, header)
			}
		})
	}
}

func TestInitClientInvalidStore(t *testing.T) {
	command := &cobra.Command{}
	command.Flags().AddFlagSet(RPCFlags())
	require.NoError(t, command.Flags().Set("url", "http://127.0.0.1:1"))
	require.NoError(t, command.Flags().Set(nodeStoreFlag, t.TempDir()))
	command.SetContext(context.Background())
	require.ErrorContains(t, InitClient(command, nil), "config.toml")
}
