package p2p

import (
	"context"
	"errors"
	"testing"

	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/celestiaorg/celestia-node/libs/keystore"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func testModule(tp node.Type) fx.Option {
	cfg := DefaultConfig(tp)
	cfg.ListenAddresses = []string{"/ip4/127.0.0.1/tcp/0"}
	// TODO(@Wondertan): Most of these can be deduplicated
	//  by moving Store into the modnode and introducing there a TestModNode module
	//  that testers would import
	opts := fx.Options(
		fx.NopLogger,
		ConstructModule(tp, &cfg),
		fx.Provide(context.Background),
		fx.Supply(Private),
		fx.Supply(Bootstrappers{}),
		fx.Supply(tp),
		fx.Provide(keystore.NewMapKeystore),
		fx.Supply(fx.Annotate(ds_sync.MutexWrap(datastore.NewMapDatastore()), fx.As(new(datastore.Batching)))),
	)
	return opts
}

func TestModuleBuild(t *testing.T) {
	test := []struct {
		tp node.Type
	}{
		{tp: node.Bridge},
		{tp: node.Light},
	}

	for _, tt := range test {
		t.Run(tt.tp.String(), func(t *testing.T) {
			var host HostBase
			var module Module
			app := fxtest.New(t, testModule(tt.tp), fx.Populate(&host, &module))
			require.Empty(t, host.Network().ListenAddresses())
			app.RequireStart()
			require.NotEmpty(t, host.Network().ListenAddresses())
			app.RequireStop()
			require.Empty(t, host.Network().ListenAddresses())
		})
	}
}

func TestModuleStartFailureClosesListeners(t *testing.T) {
	startErr := errors.New("start failed")
	listenersStarted := false
	var host HostBase
	app := fxtest.New(
		t,
		testModule(node.Light),
		fx.Populate(&host),
		fx.Invoke(func(lc fx.Lifecycle) {
			lc.Append(fx.Hook{OnStart: func(context.Context) error {
				listenersStarted = len(host.Network().ListenAddresses()) > 0
				return startErr
			}})
		}),
	)

	err := app.Start(t.Context())
	require.ErrorIs(t, err, startErr)
	require.True(t, listenersStarted)
	require.Empty(t, host.Network().ListenAddresses())
}

func TestModuleRejectsInvalidListenAddress(t *testing.T) {
	cfg := DefaultConfig(node.Light)
	cfg.ListenAddresses = []string{"invalid"}

	app := fx.New(fx.NopLogger, ConstructModule(node.Light, &cfg))
	require.ErrorContains(t, app.Err(), "failure to parse config.P2P.ListenAddresses")
}

func TestModuleBuild_WithMetrics(t *testing.T) {
	test := []struct {
		tp node.Type
	}{
		{tp: node.Bridge},
		{tp: node.Light},
	}

	for _, tt := range test {
		t.Run(tt.tp.String(), func(t *testing.T) {
			app := fxtest.New(t, testModule(tt.tp), WithMetrics())
			app.RequireStart()
			app.RequireStop()
		})
	}
}
