package shrex

import (
	"context"
	"io"
	"maps"

	"github.com/celestiaorg/celestia-node/share/shwap"
)

type newRequest func() request

func newRequestID() map[string]newRequest {
	nsDataInitID := func() request {
		return &shwap.NamespaceDataID{}
	}

	edsInitID := func() request {
		return &shwap.EdsID{}
	}

	sampleIDs := func() request { return &shwap.SampleIDs{} }

	rowID := func() request { return &shwap.RowID{} }

	request := make(map[string]newRequest)
	request[nsDataInitID().Name()] = nsDataInitID
	request[edsInitID().Name()] = edsInitID
	request[sampleIDs().Name()] = rowID
	request[rowID().Name()] = rowID
	return request
}

// SupportedProtocols returns  a slice of protocol names that
// the client and server support by default.
func SupportedProtocols() []string {
	req := newRequestID()
	protocolNames := make([]string, 0, len(req))
	for name := range maps.Keys(req) {
		protocolNames = append(protocolNames, name)
	}
	return protocolNames
}

// request represents compatible generalised interface type for shwap requests.
type request interface {
	io.WriterTo
	io.ReaderFrom

	Name() string
	// Height reports the target height of the shwap container.
	Height() uint64
	// Validate performs a basic validation of the request.
	Validate() error

	// ContainerDataReader returns io.Reader that reads data from the Accessor.
	ContainerDataReader(ctx context.Context, acc shwap.Accessor) (io.Reader, error)
}

// response compatible generalised interface type for shwap responses
type response interface {
	io.ReaderFrom
}
