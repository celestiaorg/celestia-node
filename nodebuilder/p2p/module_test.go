package p2p

import (
	"context"
	"testing"

	"github.com/ipfs/go-datastore"
	ds_sync "github.com/ipfs/go-datastore/sync"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/celestiaorg/celestia-node/libs/keystore"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func testModule(tp node.Type) fx.Option {
	cfg := DefaultConfig(tp)
	// TODO(@Wondertan): Most of these can be deduplicated
	//  by moving Store into the modnode and introducing there a TestModNode module
	//  that testers would import
	return fx.Options(
		fx.NopLogger,
		ConstructModule(tp, &cfg),
		fx.Provide(context.Background),
		fx.Supply(Private),
		fx.Supply(Bootstrappers{}),
		fx.Supply(tp),
		fx.Provide(keystore.NewMapKeystore),
		fx.Supply(fx.Annotate(ds_sync.MutexWrap(datastore.NewMapDatastore()), fx.As(new(datastore.Batching)))),
	)
}

func TestModuleBuild(t *testing.T) {
	test := []struct {
		tp node.Type
	}{
		{tp: node.Bridge},
		{tp: node.Full},
		{tp: node.Light},
	}

	for _, tt := range test {
		t.Run(tt.tp.String(), func(t *testing.T) {
			app := fxtest.New(t, testModule(tt.tp))
			app.RequireStart()
			app.RequireStop()
		})
	}
}

func TestModuleBuild_WithMetrics(t *testing.T) {
	test := []struct {
		tp node.Type
	}{
		{tp: node.Full},
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
