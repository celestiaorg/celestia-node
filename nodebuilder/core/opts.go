package core

import (
	"go.uber.org/fx"
	"google.golang.org/grpc"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
)

// WithConnection sets a custom client for core process
func WithConnection(conn *grpc.ClientConn) fx.Option {
	return fxutil.ReplaceAs(conn, new(grpc.ClientConn))
}

// WithHeaderConstructFn sets custom func that creates extended header
func WithHeaderConstructFn(construct header.ConstructFn) fx.Option {
	return fx.Replace(construct)
}
