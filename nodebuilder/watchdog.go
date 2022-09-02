package nodebuilder

import (
	"context"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/raulk/go-watchdog"
	"go.uber.org/fx"
)

var (
	// TODO(@Wondetan): We must start watchdog only once. This is needed for tests where we run multiple instance
	//  of the Node. Ideally, the Node should have some testing options instead, so we can check for it and run without
	//  such utilities but it does not hurt to run one instance of watchdog per test.
	onceWatchdog = sync.Once{}
	logWatchdog  = logging.Logger("watchdog")
)

// invokeWatchdog starts the memory watchdog that helps to prevent some of OOMs by forcing GCing
// It also collects heap profiles in the given directory when heap grows to more than 90% of memory usage
func invokeWatchdog(pprofdir string) func(lc fx.Lifecycle) error {
	return func(lc fx.Lifecycle) (errOut error) {
		onceWatchdog.Do(func() {
			// to get watchdog information logged out
			watchdog.Logger = logWatchdog
			// these set up heap pprof auto capturing on disk when threshold hit 90% usage
			watchdog.HeapProfileDir = pprofdir
			watchdog.HeapProfileMaxCaptures = 10
			watchdog.HeapProfileThreshold = 0.9

			policy := watchdog.NewWatermarkPolicy(0.50, 0.60, 0.70, 0.85, 0.90, 0.925, 0.95)
			err, stop := watchdog.SystemDriven(0, time.Second*5, policy)
			if err != nil {
				errOut = err
				return
			}

			lc.Append(fx.Hook{
				OnStop: func(context.Context) error {
					stop()
					return nil
				},
			})
		})
		return
	}
}
