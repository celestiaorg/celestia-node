package core

import (
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/share/eds"
)

// WithClient sets custom client for core process
func WithClient(client core.Client) fx.Option {
	return fxutil.ReplaceAs(client, new(core.Client))
}

// WithHeaderConstructFn sets custom func that creates extended header
func WithHeaderConstructFn(construct header.ConstructFn) fx.Option {
	return fx.Replace(construct)
}

// WithEdsConstructFn sets custom func that creates extended data square
func WithEdsConstructFn(construct eds.ConstructFn) fx.Option {
	return fx.Replace(construct)
}
