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

	request := make(map[string]newRequest)
	request[nsDataInitID().Name()] = nsDataInitID
	request[edsInitID().Name()] = edsInitID
	return request
}

type SupportedProtocolName string

func (s SupportedProtocolName) String() string {
	return string(s)
}

// SupportedProtocols returns  a slice of protocol names that
// the client and server support by default.
func SupportedProtocols() []SupportedProtocolName {
	req := newRequestID()
	protocolNames := make([]SupportedProtocolName, 0, len(req))
	for name := range maps.Keys(req) {
		protocolNames = append(protocolNames, SupportedProtocolName(name))
	}
	return protocolNames
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

	// DataReader returns io.Reader that reads data from the Accessor.
	DataReader(ctx context.Context, acc shwap.Accessor) (io.Reader, error)
}

// response compatible generalised interface type for responses
type response interface {
	io.ReaderFrom
}
