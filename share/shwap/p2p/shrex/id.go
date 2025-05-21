package shrex

import (
	"context"
	"io"
	"maps"

	"github.com/celestiaorg/celestia-node/share/shwap"
)

type newID func() id

func newInitID() map[string]newID {
	nsDataInitID := func() id {
		return &shwap.NamespaceDataID{}
	}

	edsInitID := func() id {
		return &shwap.EdsID{}
	}

	initID := make(map[string]newID)
	initID[nsDataInitID().Name()] = nsDataInitID
	initID[edsInitID().Name()] = edsInitID
	return initID
}

// SupportedProtocols returns  a slice of protocol names that
// the client and server support by default.
func SupportedProtocols() []string {
	initID := newInitID()
	protocolNames := make([]string, 0, len(initID))
	for name := range maps.Keys(initID) {
		protocolNames = append(protocolNames, name)
	}
	return protocolNames
}

// id represents compatible generalised interface type for shwap requests.
type id interface {
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

// represents compatible generalised interface type for shwap responses
type container interface {
	io.ReaderFrom
}
