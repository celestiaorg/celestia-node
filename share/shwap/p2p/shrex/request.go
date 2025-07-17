package shrex

import (
	"context"
	"io"

	"github.com/celestiaorg/celestia-node/share/shwap"
)

type newRequestID func() request

func init() {
	nsDataInitID := func() request {
		return &shwap.NamespaceDataID{}
	}
	edsInitID := func() request {
		return &shwap.EdsID{}
	}

	registry = make(map[string]newRequestID)
	registry[nsDataInitID().Name()] = nsDataInitID
	registry[edsInitID().Name()] = edsInitID
}

// registry maps protocol names to their corresponding request factory functions.
// It provides a centralized lookup for creating request instances of supported
// protocol types at runtime.
var registry map[string]newRequestID

// request represents compatible generalised interface for requests.
type request interface {
	io.WriterTo
	io.ReaderFrom

	Name() string
	// Height reports the target height of the shwap container.
	Height() uint64
	// Validate performs a basic validation of the request.
	Validate() error

	// ResponseReader returns io.Reader that reads data from the Accessor.
	ResponseReader(ctx context.Context, acc shwap.Accessor) (io.Reader, error)
}

// response compatible generalised interface type for responses
type response interface {
	io.ReaderFrom
}
