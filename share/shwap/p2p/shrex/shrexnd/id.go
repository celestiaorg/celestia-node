package shrexnd

import (
	"context"
	"io"

	"github.com/celestiaorg/celestia-node/share/shwap"
)

var initID = map[string]id{
	shwap.NamespaceDataName: &shwap.NamespaceDataID{},
	shwap.EDSName:           &shwap.EdsID{},
}

func SupportedProtocols() []string {
	return []string{shwap.NamespaceDataName, shwap.EDSName}
}

// id represents compatible generalised interface type for all shwap requests
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

// represents compatible generalised interface type for all shwap responses
type container interface {
	io.ReaderFrom
}
