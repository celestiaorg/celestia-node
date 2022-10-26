package nodebuilder

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/api/mocks"
	"github.com/celestiaorg/celestia-node/api/rpc"
	"github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/state"
)

func TestRPCCallsUnderlyingNode(t *testing.T) {
	nd, server := setupNodeWithModifiedRPC(t)
	url := nd.RPCServer.ListenAddr()
	client, closer, err := client.NewClient(context.Background(), "http://"+url)
	require.NoError(t, err)
	defer closer.CloseAll()
	ctx := context.Background()

	expectedBalance := &state.Balance{
		Amount: sdk.NewInt(100),
		Denom:  "utia",
	}

	server.EXPECT().Balance(gomock.Any()).Return(expectedBalance, nil).Times(1)

	balance, err := client.State.Balance(ctx)
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
			require.Equal(t, method.Type, impl.Type)
		}
	}
}

// TODO(@distractedm1nd): Blocked by issues #1208 and #1207
func TestAllReturnValuesAreMarshalable(t *testing.T) {
	t.Skip()
	ra := reflect.TypeOf(new(client.API)).Elem()
	for i := 0; i < ra.NumMethod(); i++ {
		m := ra.Method(i)
		for j := 0; j < m.Type.NumOut(); j++ {
			implementsMarshaler(t, m.Type.Out(j))
		}
	}
}

func implementsMarshaler(t *testing.T, typ reflect.Type) { //nolint:unused
	switch typ.Kind() {
	case reflect.Struct:
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

func setupNodeWithModifiedRPC(t *testing.T) (*Node, *mocks.MockAPI) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ctrl := gomock.NewController(t)
	server := mocks.NewMockAPI(ctrl)

	overrideRPCHandler := fx.Invoke(func(srv *rpc.Server) {
		srv.RegisterService("state", server)
		srv.RegisterService("share", server)
		srv.RegisterService("fraud", server)
		srv.RegisterService("header", server)
		srv.RegisterService("das", server)
	})
	nd := TestNode(t, node.Full, overrideRPCHandler)
	// start node
	err := nd.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		err = nd.Stop(ctx)
		require.NoError(t, err)
	})
	return nd, server
}
