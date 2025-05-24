package api

import (
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/cristalhq/jwt/v5"
	"github.com/golang/mock/gomock"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/api/rpc"
	"github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/api/rpc/perms"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/blob"
	"github.com/celestiaorg/celestia-node/nodebuilder/blobstream"
	"github.com/celestiaorg/celestia-node/nodebuilder/da"
	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	daspkg "github.com/celestiaorg/celestia-node/nodebuilder/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/modname"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
	statemod "github.com/celestiaorg/celestia-node/nodebuilder/state"
	blobMock "github.com/celestiaorg/celestia-node/nodebuilder/tests/mocks/blob"
	blobstreamMock "github.com/celestiaorg/celestia-node/nodebuilder/tests/mocks/blobstream"
	daMock "github.com/celestiaorg/celestia-node/nodebuilder/tests/mocks/da"
	dasMock "github.com/celestiaorg/celestia-node/nodebuilder/tests/mocks/das"
	fraudMock "github.com/celestiaorg/celestia-node/nodebuilder/tests/mocks/fraud"
	headerMock "github.com/celestiaorg/celestia-node/nodebuilder/tests/mocks/header"
	nodeMock "github.com/celestiaorg/celestia-node/nodebuilder/tests/mocks/node"
	p2pMock "github.com/celestiaorg/celestia-node/nodebuilder/tests/mocks/p2p"
	shareMock "github.com/celestiaorg/celestia-node/nodebuilder/tests/mocks/share"
	stateMock "github.com/celestiaorg/celestia-node/nodebuilder/tests/mocks/state"
	"github.com/celestiaorg/celestia-node/state"
)

func TestRPCCallsUnderlyingNode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// generate dummy signer and sign admin perms token with it
	key := make([]byte, 32)

	signer, err := jwt.NewSignerHS(jwt.HS256, key)
	require.NoError(t, err)

	verifier, err := jwt.NewVerifierHS(jwt.HS256, key)
	require.NoError(t, err)

	nd, server := setupNodeWithAuthedRPC(t, signer, verifier)
	url := nd.RPCServer.ListenAddr()

	adminToken, err := perms.NewTokenWithPerms(signer, perms.AllPerms)
	require.NoError(t, err)

	// we need to run this a few times to prevent the race where the server is not yet started
	var (
		rpcClient *client.Client
	)
	for range 3 {
		time.Sleep(time.Second * 1)
		rpcClient, err = client.NewClient(ctx, "http://"+url, string(adminToken))
		if err == nil {
			t.Cleanup(rpcClient.Close)
			break
		}
	}
	require.NotNil(t, rpcClient)
	require.NoError(t, err)

	expectedBalance := &state.Balance{
		Amount: math.NewInt(100),
		Denom:  "utia",
	}

	server.State.EXPECT().Balance(gomock.Any()).Return(expectedBalance, nil)

	balance, err := rpcClient.State.Balance(ctx)
	require.NoError(t, err)
	require.Equal(t, expectedBalance, balance)
}

func TestRPCCallsTokenExpired(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// generate dummy signer and sign admin perms token with it
	key := make([]byte, 32)

	signer, err := jwt.NewSignerHS(jwt.HS256, key)
	require.NoError(t, err)

	verifier, err := jwt.NewVerifierHS(jwt.HS256, key)
	require.NoError(t, err)

	nd, _ := setupNodeWithAuthedRPC(t, signer, verifier)
	url := nd.RPCServer.ListenAddr()

	adminToken, err := perms.NewTokenWithTTL(signer, perms.AllPerms, time.Millisecond)
	require.NoError(t, err)

	// we need to run this a few times to prevent the race where the server is not yet started
	var rpcClient *client.Client
	for range 3 {
		time.Sleep(time.Second * 1)
		rpcClient, err = client.NewClient(ctx, "http://"+url, string(adminToken))
		if err == nil {
			t.Cleanup(rpcClient.Close)
			break
		}
	}
	require.NotNil(t, rpcClient)
	require.NoError(t, err)

	_, err = rpcClient.State.Balance(ctx)
	require.ErrorContains(t, err, "request failed, http status 401 Unauthorized")
}

// api contains all modules that are made available as the node's
// public API surface
type api struct {
	Fraud      fraud.Module
	Header     header.Module
	State      statemod.Module
	Share      share.Module
	DAS        das.Module
	Node       node.Module
	P2P        p2p.Module
	Blob       blob.Module
	DA         da.Module
	Blobstream blobstream.Module
}

func TestModulesImplementFullAPI(t *testing.T) {
	api := reflect.TypeOf(new(api)).Elem()
	client := reflect.TypeOf(new(client.Client)).Elem()
	for i := range client.NumField() {
		module := client.Field(i)
		switch module.Name {
		case "closer":
			// the "closers" field is not an actual module
			continue
		default:
			internal, ok := module.Type.FieldByName("Internal")
			require.True(t, ok, "module %s's API does not have an Internal field", module.Name)
			for j := range internal.Type.NumField() {
				impl := internal.Type.Field(j)
				field, _ := api.FieldByName(module.Name)
				method, _ := field.Type.MethodByName(impl.Name)
				require.Equal(t, method.Type, impl.Type, "method %s does not match", impl.Name)
			}
		}
	}
}

func TestAuthedRPC(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// generate dummy signer and sign admin perms token with it
	key := make([]byte, 32)

	signer, err := jwt.NewSignerHS(jwt.HS256, key)
	require.NoError(t, err)

	verifier, err := jwt.NewVerifierHS(jwt.HS256, key)
	require.NoError(t, err)

	nd, server := setupNodeWithAuthedRPC(t, signer, verifier)
	url := nd.RPCServer.ListenAddr()

	// create permissioned tokens
	publicToken, err := perms.NewTokenWithPerms(signer, perms.DefaultPerms)
	require.NoError(t, err)
	readToken, err := perms.NewTokenWithPerms(signer, perms.ReadPerms)
	require.NoError(t, err)
	rwToken, err := perms.NewTokenWithPerms(signer, perms.ReadWritePerms)
	require.NoError(t, err)
	adminToken, err := perms.NewTokenWithPerms(signer, perms.AllPerms)
	require.NoError(t, err)

	tests := []struct {
		perm  int
		token string
	}{
		{perm: 1, token: string(publicToken)}, // public
		{perm: 2, token: string(readToken)},   // read
		{perm: 3, token: string(rwToken)},     // RW
		{perm: 4, token: string(adminToken)},  // admin
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			// we need to run this a few times to prevent the race where the server is not yet started
			var rpcClient *client.Client
			require.NoError(t, err)
			for range 3 {
				time.Sleep(time.Second * 1)
				rpcClient, err = client.NewClient(ctx, "http://"+url, tt.token)
				if err == nil {
					break
				}
			}
			require.NotNil(t, rpcClient)
			require.NoError(t, err)

			// 1. Test method with read-level permissions
			expected := daspkg.SamplingStats{
				SampledChainHead: 100,
				CatchupHead:      100,
				NetworkHead:      1000,
				Failed:           nil,
				Workers:          nil,
				Concurrency:      0,
				CatchUpDone:      true,
				IsRunning:        false,
			}
			if tt.perm > 1 {
				server.Das.EXPECT().SamplingStats(gomock.Any()).Return(expected, nil)
				stats, err := rpcClient.DAS.SamplingStats(ctx)
				require.NoError(t, err)
				require.Equal(t, expected, stats)
			} else {
				_, err := rpcClient.DAS.SamplingStats(ctx)
				require.Error(t, err)
				require.ErrorContains(t, err, "missing permission")
			}

			// 2. Test method with write-level permissions
			expectedResp := &state.TxResponse{}
			if tt.perm > 2 {
				server.State.EXPECT().Delegate(gomock.Any(), gomock.Any(),
					gomock.Any(), gomock.Any()).Return(expectedResp, nil)
				txResp, err := rpcClient.State.Delegate(ctx,
					state.ValAddress{}, state.Int{}, state.NewTxConfig())
				require.NoError(t, err)
				require.Equal(t, expectedResp, txResp)
			} else {
				_, err := rpcClient.State.Delegate(ctx,
					state.ValAddress{}, state.Int{}, state.NewTxConfig())
				require.Error(t, err)
				require.ErrorContains(t, err, "missing permission")
			}

			// 3. Test method with admin-level permissions
			expectedReachability := network.Reachability(3)
			if tt.perm > 3 {
				server.P2P.EXPECT().NATStatus(gomock.Any()).Return(expectedReachability, nil)
				natstatus, err := rpcClient.P2P.NATStatus(ctx)
				require.NoError(t, err)
				require.Equal(t, expectedReachability, natstatus)
			} else {
				_, err := rpcClient.P2P.NATStatus(ctx)
				require.Error(t, err)
				require.ErrorContains(t, err, "missing permission")
			}

			rpcClient.Close()
		})
	}
}

func TestAllReturnValuesAreMarshalable(t *testing.T) {
	ra := reflect.TypeOf(new(api)).Elem()
	for i := range ra.NumMethod() {
		m := ra.Method(i)
		for j := range m.Type.NumOut() {
			implementsMarshaler(t, m.Type.Out(j))
		}
	}
	// NOTE: see comment above api interface definition.
	na := reflect.TypeOf(new(node.Module)).Elem()
	for i := range na.NumMethod() {
		m := na.Method(i)
		for j := range m.Type.NumOut() {
			implementsMarshaler(t, m.Type.Out(j))
		}
	}
}

func implementsMarshaler(t *testing.T, typ reflect.Type) {
	// the passed type may already implement json.Marshaler and we don't need to go deeper
	if typ.Implements(reflect.TypeOf(new(json.Marshaler)).Elem()) {
		return
	}

	switch typ.Kind() {
	case reflect.Struct:
		// a user defined struct could implement json.Marshaler on the pointer receiver, so check there
		// first. note that the "non-pointer" receiver is checked before the switch.
		pointerType := reflect.TypeOf(reflect.New(typ).Elem().Addr().Interface())
		if pointerType.Implements(reflect.TypeOf(new(json.Marshaler)).Elem()) {
			return
		}
		// struct doesn't implement the interface itself, check all individual fields
		reflect.New(typ).Pointer()
		for i := range typ.NumField() {
			implementsMarshaler(t, typ.Field(i).Type)
		}
		return
	case reflect.Map:
		implementsMarshaler(t, typ.Elem())
		implementsMarshaler(t, typ.Key())
	case reflect.Ptr:
		fallthrough
	case reflect.Array:
		fallthrough
	case reflect.Slice:
		fallthrough
	case reflect.Chan:
		implementsMarshaler(t, typ.Elem())
	case reflect.Interface:
		if typ != reflect.TypeOf(new(any)).Elem() && typ != reflect.TypeOf(new(error)).Elem() {
			require.True(
				t,
				typ.Implements(reflect.TypeOf(new(json.Marshaler)).Elem()),
				"type %s does not implement json.Marshaler", typ.String(),
			)
		}
	default:
		return
	}
}

// setupNodeWithAuthedRPC sets up a node and overrides its JWT
// signer with the given signer.
func setupNodeWithAuthedRPC(t *testing.T,
	jwtSigner jwt.Signer, jwtVerifier jwt.Verifier,
) (*nodebuilder.Node, *mockAPI) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

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

	// given the behavior of fx.Invoke, this invoke will be called last as it is added at the root
	// level module. For further information, check the documentation on fx.Invoke.
	invokeRPC := fx.Invoke(func(srv *rpc.Server) {
		srv.RegisterService(modname.Fraud, mockAPI.Fraud, &fraud.API{})
		srv.RegisterService(modname.DAS, mockAPI.Das, &das.API{})
		srv.RegisterService(modname.Header, mockAPI.Header, &header.API{})
		srv.RegisterService(modname.State, mockAPI.State, &statemod.API{})
		srv.RegisterService(modname.Share, mockAPI.Share, &share.API{})
		srv.RegisterService(modname.P2P, mockAPI.P2P, &p2p.API{})
		srv.RegisterService(modname.Node, mockAPI.Node, &node.API{})
		srv.RegisterService(modname.Blob, mockAPI.Blob, &blob.API{})
		srv.RegisterService(modname.DA, mockAPI.DA, &da.API{})
	})
	// fx.Replace does not work here, but fx.Decorate does
	nd := nodebuilder.TestNode(t, node.Full, invokeRPC, fx.Decorate(func() (jwt.Signer, jwt.Verifier, error) {
		return jwtSigner, jwtVerifier, nil
	}))
	// start node
	err := nd.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		err = nd.Stop(ctx)
		require.NoError(t, err)
	})
	return nd, mockAPI
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
