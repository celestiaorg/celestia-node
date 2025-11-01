package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	coregrpc "github.com/cometbft/cometbft/rpc/grpc"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cristalhq/jwt/v5"
	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-app/v6/test/util/testnode"
	libshare "github.com/celestiaorg/go-square/v3/share"

	"github.com/celestiaorg/celestia-node/api/rpc"
	"github.com/celestiaorg/celestia-node/api/rpc/perms"
	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	blobapi "github.com/celestiaorg/celestia-node/nodebuilder/blob"
	blobMock "github.com/celestiaorg/celestia-node/nodebuilder/blob/mocks"
	blobstreamMock "github.com/celestiaorg/celestia-node/nodebuilder/blobstream/mocks"
	"github.com/celestiaorg/celestia-node/nodebuilder/da"
	daMock "github.com/celestiaorg/celestia-node/nodebuilder/da/mocks"
	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	dasMock "github.com/celestiaorg/celestia-node/nodebuilder/das/mocks"
	"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	fraudMock "github.com/celestiaorg/celestia-node/nodebuilder/fraud/mocks"
	headerapi "github.com/celestiaorg/celestia-node/nodebuilder/header"
	headerMock "github.com/celestiaorg/celestia-node/nodebuilder/header/mocks"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	nodeMock "github.com/celestiaorg/celestia-node/nodebuilder/node/mocks"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	p2pMock "github.com/celestiaorg/celestia-node/nodebuilder/p2p/mocks"
	"github.com/celestiaorg/celestia-node/nodebuilder/share"
	shareMock "github.com/celestiaorg/celestia-node/nodebuilder/share/mocks"
	statemod "github.com/celestiaorg/celestia-node/nodebuilder/state"
	stateMock "github.com/celestiaorg/celestia-node/nodebuilder/state/mocks"
	"github.com/celestiaorg/celestia-node/state"
)

// TestClientReadOnlyMode tests the client in read-only mode
func TestClientReadOnlyMode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	// Setup mock server with authed RPC
	server, mockAPI, authToken, cleanup := setupMockRPCServer(t, ctx)
	defer cleanup()

	// Get the server URL
	serverURL := "http://" + server.RPCServer.ListenAddr()

	// Temporary directory for client storage
	tmpDir, err := os.MkdirTemp("", "celestia-client-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create client config in read-only mode
	cfg := ReadConfig{
		BridgeDAAddr: serverURL,
		DAAuthToken:  authToken,
		EnableDATLS:  false,
	}

	// Initialize client
	client, err := NewReadClient(ctx, cfg)
	require.NoError(t, err)
	defer client.Close()

	// Test that the client is properly initialized
	require.NotNil(t, client.Header)
	require.NotNil(t, client.Share)
	require.NotNil(t, client.Blob)
	require.NotNil(t, client.Blobstream)

	// Configure mock expectations for read operations
	mockHeader := &header.ExtendedHeader{}
	mockAPI.Header.EXPECT().GetByHeight(gomock.Any(), gomock.Any()).Return(mockHeader, nil).Times(1)

	// Test header read operation works
	_, err = client.Header.GetByHeight(ctx, 1)
	require.NoError(t, err)

	// Mock blob get operation
	namespace := libshare.MustNewV0Namespace(bytes.Repeat([]byte{0xb}, 10))
	b, err := blob.NewBlob(libshare.ShareVersionZero, namespace, []byte("dataA"), nil)
	require.NoError(t, err)
	mockAPI.Blob.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(b, nil).Times(1)

	// Test blob read operation works
	blobs, err := client.Blob.Get(ctx, 1, namespace, b.Commitment)
	require.NoError(t, err)
	require.Equal(t, b, blobs)

	// Test blob.Submit returns an error in read-only mode
	submitBlobs := []*blob.Blob{b}
	_, err = client.Blob.Submit(ctx, submitBlobs, nil)
	require.ErrorIs(t, err, ErrReadOnlyMode)
}

// TestClientWithSubmission tests the client with submission capabilities
func TestSubmission(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)

	accounts := []string{
		"Elon",
		"Mark",
		"Jeff",
		"Warren",
	}

	start := time.Now()
	cctx := setupConsensus(t, ctx, accounts...)
	fmt.Println("consensus", time.Since(start).String())

	bn, adminToken := bridgeNode(t, ctx, cctx)
	fmt.Println("bridgeNode", time.Since(start).String())

	cfg := Config{
		ReadConfig: ReadConfig{
			BridgeDAAddr: "http://" + bn.RPCServer.ListenAddr(),
			DAAuthToken:  adminToken,
			EnableDATLS:  false,
		},

		SubmitConfig: SubmitConfig{
			DefaultKeyName: accounts[0],
			Network:        "private",
			CoreGRPCConfig: CoreGRPCConfig{
				Addr: cctx.GRPCClient.Target(),
			},
		},
	}
	client, err := New(ctx, cfg, cctx.Keyring)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})

	// Test State returns actual balance
	balance, err := client.State.Balance(ctx)
	require.NoError(t, err)
	fmt.Println("balance", balance)

	// Test header read operation works
	_, err = client.Header.GetByHeight(ctx, 1)
	require.NoError(t, err)

	namespace := libshare.MustNewV0Namespace(bytes.Repeat([]byte{0xb}, 10))
	b, err := blob.NewBlob(libshare.ShareVersionZero, namespace, []byte("dataA"), nil)
	require.NoError(t, err)
	submitBlobs := []*blob.Blob{b}

	now := time.Now()
	// Submit a blob from default key
	height, err := client.Blob.Submit(ctx, submitBlobs, &blob.SubmitOptions{})
	require.NoError(t, err)
	fmt.Println("height default", height, time.Since(now).String())

	// Test blob read operation works
	received, err := client.Blob.Get(ctx, height, namespace, b.Commitment)
	require.NoError(t, err)
	require.Equal(t, b.Data(), received.Data())
	fmt.Println("get default", time.Since(now).String())

	// Submit a blob from non-default key
	for _, acc := range accounts[1:] {
		// submit takes ~3 seconds to confirm each Tx on localnet
		submitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		t.Cleanup(cancel)

		submitCfg := state.NewTxConfig(
			state.WithKeyName(acc),
		)
		height, err := client.Blob.Submit(submitCtx, submitBlobs, submitCfg)
		require.NoError(t, err)
		fmt.Println("submit", acc, height, time.Since(now).String())

		received, err := client.Blob.Get(submitCtx, height, namespace, b.Commitment)
		require.NoError(t, err)
		require.Equal(t, b.Data(), received.Data())
		fmt.Println("get", acc, time.Since(now).String())
	}
}

type mockAPI struct {
	State      *stateMock.MockModule
	Share      *shareMock.MockModule
	Fraud      *fraudMock.MockModule
	Header     *headerMock.MockModule
	Das        *dasMock.MockModule
	P2P        *p2pMock.MockModule
	Node       *nodeMock.MockModule
	Blob       *blobMock.MockModule
	DA         *daMock.MockModule
	Blobstream *blobstreamMock.MockModule
}

// setupMockRPCServer creates a mock JSON-RPC server using the actual server implementation with
// mocked components
func setupMockRPCServer(t *testing.T, ctx context.Context) (*nodebuilder.Node, *mockAPI, string, func()) {
	ctrl := gomock.NewController(t)
	mockAPI := &mockAPI{
		stateMock.NewMockModule(ctrl),
		shareMock.NewMockModule(ctrl),
		fraudMock.NewMockModule(ctrl),
		headerMock.NewMockModule(ctrl),
		dasMock.NewMockModule(ctrl),
		p2pMock.NewMockModule(ctrl),
		nodeMock.NewMockModule(ctrl),
		blobMock.NewMockModule(ctrl),
		daMock.NewMockModule(ctrl),
		blobstreamMock.NewMockModule(ctrl),
	}
	t.Cleanup(ctrl.Finish)

	// given the behavior of fx.Invoke, this invoke will be called last as it is added at the root
	// level module. For further information, check the documentation on fx.Invoke.
	invokeRPC := fx.Invoke(func(srv *rpc.Server) {
		srv.RegisterService("fraud", mockAPI.Fraud, &fraud.API{})
		srv.RegisterService("das", mockAPI.Das, &das.API{})
		srv.RegisterService("header", mockAPI.Header, &headerapi.API{})
		srv.RegisterService("state", mockAPI.State, &statemod.API{})
		srv.RegisterService("share", mockAPI.Share, &share.API{})
		srv.RegisterService("p2p", mockAPI.P2P, &p2p.API{})
		srv.RegisterService("node", mockAPI.Node, &node.API{})
		srv.RegisterService("blob", mockAPI.Blob, &blobapi.API{})
		srv.RegisterService("da", mockAPI.DA, &da.API{})
	})

	// Setup node with authenticated RPC
	addAuth, adminToken := addAuth(t)
	nd := nodebuilder.TestNode(t, node.Full, invokeRPC, addAuth)
	// start node
	err := nd.Start(ctx)
	require.NoError(t, err)
	cleanup := func() {
		err := nd.Stop(ctx)
		require.NoError(t, err)
	}

	return nd, mockAPI, adminToken, cleanup
}

// TestMultiEndpointClient tests the multi-endpoint client functionality
func TestMultiEndpointClient(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	// Setup mock server with authed RPC
	server, _, authToken, cleanup := setupMockRPCServer(t, ctx)
	defer cleanup()

	// Get the server URL
	serverURL := "http://" + server.RPCServer.ListenAddr()

	// Temporary directory for client storage
	tmpDir, err := os.MkdirTemp("", "celestia-client-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create keyring
	kr, err := KeyringWithNewKey(KeyringConfig{
		KeyName:     "test_key",
		BackendName: keyring.BackendTest,
	}, tmpDir)
	require.NoError(t, err)

	// Test that multi-endpoint client creation fails gracefully with invalid config
	invalidCfg := Config{
		ReadConfig: ReadConfig{
			BridgeDAAddr: serverURL,
			AdditionalBridgeDAAddrs: []string{
				"", // Empty address should be invalid
			},
			DAAuthToken: authToken,
			EnableDATLS: false,
		},
		SubmitConfig: SubmitConfig{
			DefaultKeyName: "test_key",
			Network:        "mocha-4",
			CoreGRPCConfig: CoreGRPCConfig{
				Addr:       "localhost:9090",
				TLSEnabled: false,
				AuthToken:  "",
			},
		},
	}
	_, err = NewMultiEndpoint(ctx, invalidCfg, kr)
	require.Error(t, err)

	// Test that multi-endpoint client creation fails gracefully with invalid gRPC config
	invalidCfg = Config{
		ReadConfig: ReadConfig{
			BridgeDAAddr: serverURL,
			DAAuthToken:  authToken,
			EnableDATLS:  false,
		},
		SubmitConfig: SubmitConfig{
			DefaultKeyName: "test_key",
			Network:        "mocha-4",
			CoreGRPCConfig: CoreGRPCConfig{
				Addr: "localhost:9090",
				AdditionalCoreGRPCConfigs: []CoreGRPCConfig{
					{
						Addr: "", // Empty address should be invalid
					},
				},
				TLSEnabled: false,
				AuthToken:  "",
			},
		},
	}
	_, err = NewMultiEndpoint(ctx, invalidCfg, kr)
	require.Error(t, err)

	// Test successful multi-read client creation (without gRPC to avoid connection issues)
	multiReadClient, err := NewMultiReadClient(ctx, ReadConfig{
		BridgeDAAddr: serverURL,
		AdditionalBridgeDAAddrs: []string{
			serverURL, // Same server for testing, in real usage would be different
		},
		DAAuthToken: authToken,
		EnableDATLS: false,
	})
	require.NoError(t, err)
	require.NotNil(t, multiReadClient)
	defer multiReadClient.Close()

	// Test that we have multiple read clients
	readClients := multiReadClient.GetAllClients()
	require.GreaterOrEqual(t, len(readClients), 1)

	// Test that the client works normally
	require.NotNil(t, readClients[0].Header)
	require.NotNil(t, readClients[0].Share)
	require.NotNil(t, readClients[0].Blob)
	require.NotNil(t, readClients[0].Blobstream)
}

// TestReadConfigValidation tests validation of ReadConfig with multiple endpoints
func TestReadConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		cfg       ReadConfig
		expectErr bool
	}{
		{
			name: "valid config with additional endpoints",
			cfg: ReadConfig{
				BridgeDAAddr: "http://localhost:26658",
				AdditionalBridgeDAAddrs: []string{
					"http://backup1:26658",
					"http://backup2:26658",
				},
				DAAuthToken: "token",
			},
			expectErr: false,
		},
		{
			name: "invalid additional endpoint",
			cfg: ReadConfig{
				BridgeDAAddr: "http://localhost:26658",
				AdditionalBridgeDAAddrs: []string{
					"", // Empty address should be invalid
				},
				DAAuthToken: "token",
			},
			expectErr: true,
		},
		{
			name: "empty additional endpoints",
			cfg: ReadConfig{
				BridgeDAAddr:            "http://localhost:26658",
				AdditionalBridgeDAAddrs: []string{},
				DAAuthToken:             "token",
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestCoreGRPCConfigValidation tests validation of CoreGRPCConfig with multiple endpoints
func TestCoreGRPCConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		cfg       CoreGRPCConfig
		expectErr bool
	}{
		{
			name: "valid config with additional endpoints",
			cfg: CoreGRPCConfig{
				Addr: "localhost:9090",
				AdditionalCoreGRPCConfigs: []CoreGRPCConfig{
					{
						Addr:       "backup1:9090",
						TLSEnabled: true,
						AuthToken:  "token",
					},
					{
						Addr:       "backup2:9090",
						TLSEnabled: false,
						AuthToken:  "",
					},
				},
				TLSEnabled: true,
				AuthToken:  "token",
			},
			expectErr: false,
		},
		{
			name: "invalid additional endpoint",
			cfg: CoreGRPCConfig{
				Addr: "localhost:9090",
				AdditionalCoreGRPCConfigs: []CoreGRPCConfig{
					{
						Addr: "", // Empty address should be invalid
					},
				},
				TLSEnabled: false,
			},
			expectErr: true,
		},
		{
			name: "empty additional endpoints",
			cfg: CoreGRPCConfig{
				Addr:                      "localhost:9090",
				AdditionalCoreGRPCConfigs: []CoreGRPCConfig{},
				TLSEnabled:                false,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func addAuth(t *testing.T) (fx.Option, string) {
	// Generate JWT signer and verifier for the server
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)

	signer, err := jwt.NewSignerHS(jwt.HS256, key)
	require.NoError(t, err)

	verifier, err := jwt.NewVerifierHS(jwt.HS256, key)
	require.NoError(t, err)

	// Create an admin token to authenticate requests
	adminToken, err := perms.NewTokenWithPerms(signer, perms.AllPerms)
	require.NoError(t, err)
	return fx.Decorate(func() (jwt.Signer, jwt.Verifier, error) {
		return signer, verifier, nil
	}), string(adminToken)
}

func setupConsensus(t *testing.T, ctx context.Context, accounts ...string) testnode.Context {
	t.Helper()
	chainID := "private"

	config := testnode.DefaultConfig().
		WithChainID(chainID).
		WithFundedAccounts(accounts...).
		WithTimeoutCommit(1 * time.Millisecond).
		WithDelayedPrecommitTimeout(50 * time.Millisecond)

	cctx, _, _ := testnode.NewNetwork(t, config)

	bAPI := coregrpc.NewBlockAPIClient(cctx.GRPCClient)
	cl, err := bAPI.BlockByHeight(ctx, &coregrpc.BlockByHeightRequest{Height: 2})
	require.NoError(t, err)
	_, err = cl.Recv()
	require.NoError(t, err)
	return cctx
}

func bridgeNode(t *testing.T, ctx context.Context, cctx testnode.Context) (*nodebuilder.Node, string) {
	t.Helper()
	cfg := nodebuilder.DefaultConfig(node.Bridge)

	ip, port, err := net.SplitHostPort(cctx.GRPCClient.Target())
	require.NoError(t, err)

	cfg.Core.IP = ip
	cfg.Core.Port = port
	cfg.RPC.Port = "0"

	tempDir := t.TempDir()
	store := nodebuilder.MockStore(t, cfg)
	addAuth, adminToken := addAuth(t)

	// generate new keyring for BN to prevent it from signing with client keys
	keysCfg := KeyringConfig{
		KeyName:     "my_celes_key",
		BackendName: keyring.BackendTest,
	}
	kr, err := KeyringWithNewKey(keysCfg, tempDir)
	require.NoError(t, err)

	bn, err := nodebuilder.New(node.Bridge, p2p.Private, store,
		addAuth,
		statemod.WithKeyring(kr),
		statemod.WithKeyName(statemod.AccountName(keysCfg.KeyName)),
		fx.Replace(node.StorePath(tempDir)),
	)
	require.NoError(t, err)

	err = bn.Start(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		require.NoError(t, bn.Stop(ctx))
		cancel()
	})
	return bn, adminToken
}
