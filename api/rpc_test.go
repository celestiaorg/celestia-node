package api

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/api/rpc"
	"github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	dasMock "github.com/celestiaorg/celestia-node/nodebuilder/das/mocks"
	"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	fraudMock "github.com/celestiaorg/celestia-node/nodebuilder/fraud/mocks"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
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

func TestRPCCallsUnderlyingNode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	nd, server := setupNodeWithModifiedRPC(t)
	url := nd.RPCServer.ListenAddr()
	// we need to run this a few times to prevent the race where the server is not yet started
	var (
		rpcClient *client.Client
		err       error
	)
	for i := 0; i < 3; i++ {
		time.Sleep(time.Second * 1)
		rpcClient, err = client.NewClient(ctx, "http://"+url)
		if err == nil {
			t.Cleanup(rpcClient.Close)
			break
		}
	}
	require.NotNil(t, rpcClient)
	require.NoError(t, err)

	expectedBalance := &state.Balance{
		Amount: sdk.NewInt(100),
		Denom:  "utia",
	}

	server.State.EXPECT().Balance(gomock.Any()).Return(expectedBalance, nil).Times(1)

	balance, err := rpcClient.State.Balance(ctx)
	require.NoError(t, err)
	require.Equal(t, expectedBalance, balance)
}

// api contains all modules that are made available as the node's
// public API surface (except for the `node` module as the node
// module contains a method `Info` that is also contained in the
// p2p module).
type api interface {
	fraud.Module
	header.Module
	statemod.Module
	share.Module
	das.Module
	p2p.Module
}

func TestModulesImplementFullAPI(t *testing.T) {
	api := reflect.TypeOf(new(api)).Elem()
	nodeapi := reflect.TypeOf(new(node.Module)).Elem() // TODO @renaynay: explain
	client := reflect.TypeOf(new(client.Client)).Elem()
	for i := 0; i < client.NumField(); i++ {
		module := client.Field(i)
		switch module.Name {
		case "closer":
			// the "closers" field is not an actual module
			continue
		case "Node":
			// node module contains a duplicate method to the p2p module
			// and must be tested separately.
			internal, ok := module.Type.FieldByName("Internal")
			require.True(t, ok, "module %s's API does not have an Internal field", module.Name)
			for j := 0; j < internal.Type.NumField(); j++ {
				impl := internal.Type.Field(j)
				method, _ := nodeapi.MethodByName(impl.Name)
				require.Equal(t, method.Type, impl.Type, "method %s does not match", impl.Name)
			}
		default:
			internal, ok := module.Type.FieldByName("Internal")
			require.True(t, ok, "module %s's API does not have an Internal field", module.Name)
			for j := 0; j < internal.Type.NumField(); j++ {
				impl := internal.Type.Field(j)
				method, _ := api.MethodByName(impl.Name)
				require.Equal(t, method.Type, impl.Type, "method %s does not match", impl.Name)
			}
		}
	}
}

func TestAllReturnValuesAreMarshalable(t *testing.T) {
	ra := reflect.TypeOf(new(api)).Elem()
	for i := 0; i < ra.NumMethod(); i++ {
		m := ra.Method(i)
		for j := 0; j < m.Type.NumOut(); j++ {
			implementsMarshaler(t, m.Type.Out(j))
		}
	}
	// NOTE: see comment above api interface definition.
	na := reflect.TypeOf(new(node.Module)).Elem()
	for i := 0; i < na.NumMethod(); i++ {
		m := na.Method(i)
		for j := 0; j < m.Type.NumOut(); j++ {
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
		for i := 0; i < typ.NumField(); i++ {
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
		if typ != reflect.TypeOf(new(interface{})).Elem() && typ != reflect.TypeOf(new(error)).Elem() {
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

func setupNodeWithModifiedRPC(t *testing.T) (*nodebuilder.Node, *mockAPI) {
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
	}

	// given the behavior of fx.Invoke, this invoke will be called last as it is added at the root
	// level module. For further information, check the documentation on fx.Invoke.
	invokeRPC := fx.Invoke(func(srv *rpc.Server) {
		srv.RegisterService("state", mockAPI.State)
		srv.RegisterService("share", mockAPI.Share)
		srv.RegisterService("fraud", mockAPI.Fraud)
		srv.RegisterService("header", mockAPI.Header)
		srv.RegisterService("das", mockAPI.Das)
		srv.RegisterService("p2p", mockAPI.P2P)
		srv.RegisterService("node", mockAPI.Node)
	})
	nd := nodebuilder.TestNode(t, node.Full, invokeRPC)
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
	State  *stateMock.MockModule
	Share  *shareMock.MockModule
	Fraud  *fraudMock.MockModule
	Header *headerMock.MockModule
	Das    *dasMock.MockModule
	P2P    *p2pMock.MockModule
	Node   *nodeMock.MockModule
}
