package shrex

import (
	"context"
	"io"

	"github.com/celestiaorg/celestia-node/share/shwap"
)

// newRequestID is an alias to the function type that creates new request instances.
type newRequestID func() request

// registry contains factory functions for creating different types of request IDs.
// Each function returns a new instance of a specific request type that implements
// the request interface. The registry is used to instantiate the appropriate
// request type based on its type identifier
var registry = []newRequestID{
	func() request {
		return &shwap.NamespaceDataID{}
	},
	func() request {
		return &shwap.EdsID{}
	},
}

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

// response compatible generalised interface type for responses.
type response interface {
	io.ReaderFrom
}
