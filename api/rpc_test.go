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
	dasMock "github.com/celestiaorg/celestia-node/nodebuilder/das/mocks"
	fraudMock "github.com/celestiaorg/celestia-node/nodebuilder/fraud/mocks"
	headerMock "github.com/celestiaorg/celestia-node/nodebuilder/header/mocks"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	shareMock "github.com/celestiaorg/celestia-node/nodebuilder/share/mocks"
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

func TestModulesImplementFullAPI(t *testing.T) {
	api := reflect.TypeOf(new(client.API)).Elem()
	client := reflect.TypeOf(new(client.Client)).Elem()
	for i := 0; i < client.NumField(); i++ {
		module := client.Field(i)
		for j := 0; j < module.Type.NumField(); j++ {
			impl := module.Type.Field(j)
			method, _ := api.MethodByName(impl.Name)
			// closers is the only thing on the Client struct that doesn't exist in the API
			if impl.Name != "closers" {
				require.Equal(t, method.Type, impl.Type, "method %s does not match", impl.Name)
			}
		}
	}
}

func TestAllReturnValuesAreMarshalable(t *testing.T) {
	ra := reflect.TypeOf(new(client.API)).Elem()
	for i := 0; i < ra.NumMethod(); i++ {
		m := ra.Method(i)
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
	}

	// given the behavior of fx.Invoke, this invoke will be called last as it is added at the root
	// level module. For further information, check the documentation on fx.Invoke.
	invokeRPC := fx.Invoke(func(srv *rpc.Server) {
		srv.RegisterService("state", mockAPI.State)
		srv.RegisterService("share", mockAPI.Share)
		srv.RegisterService("fraud", mockAPI.Fraud)
		srv.RegisterService("header", mockAPI.Header)
		srv.RegisterService("das", mockAPI.Das)
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
}
