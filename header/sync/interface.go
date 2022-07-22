package sync

import "context"

// Sub provides a subscription to whether Syncer is
// synced up to network head.
type Sub interface {
	WaitSync(context.Context) error
	Finished() bool
}
