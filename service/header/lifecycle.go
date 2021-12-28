package header

import "context"

// Lifecycle encompasses the behavior accessible to Service
// of its sub-services.
type Lifecycle interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}
