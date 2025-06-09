package shwap

import (
	"context"
	"io"

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/share"
)

// Accessor is used to access the data from the shwap containers.
type Accessor interface {
	AxisRoots(context.Context) (*share.AxisRoots, error)
	RowNamespaceData(context.Context, libshare.Namespace, int) (RowNamespaceData, error)
	Reader() (io.Reader, error)
}
