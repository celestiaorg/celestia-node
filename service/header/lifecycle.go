package header

import "context"

// TODO @renaynay: document
type Lifecycle interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}
