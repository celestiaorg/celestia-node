package shrexnd

import (
	"context"
	"io"

	"github.com/celestiaorg/celestia-node/share/shwap"
)

var initID = map[string]func() id{
	namespaceDataID().Name(): namespaceDataID,
	edsID().Name():           edsID,
}

func namespaceDataID() id {
	return &shwap.NamespaceDataID{}
}

func edsID() id {
	return &shwap.EdsID{}
}

func SupportedProtocols() []string {
	protocolNames := make([]string, 0, len(initID))
	for name := range initID {
		protocolNames = append(protocolNames, name)
	}
	return protocolNames
}

// id represents compatible generalised interface type for shwap requests
type id interface {
	io.WriterTo
	io.ReaderFrom

	Name() string
	// Target reports the target height of the shwap container
	Target() uint64
	// Validate performs a basic validation of the request.
	Validate() error

	// FetchContainerReader gets Accessor and wrap all requested data with `io.Reader`
	FetchContainerReader(ctx context.Context, acc shwap.Accessor) (io.Reader, error)
}

// represents compatible generalised interface type for shwap responses
type container interface {
	io.ReaderFrom
}
