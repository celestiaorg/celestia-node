package availability

import (
	"errors"
	"os"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
)

const (
	SamplingWindow = 7 * 24 * time.Hour // Ref: CIP-036
	RequestWindow  = SamplingWindow
	StorageWindow  = RequestWindow + time.Hour
)

// overrideWindowEnv shrinks (or grows) the availability window for every consumer:
// sampling, header routing and pruning.
const overrideWindowEnv = "CELESTIA_OVERRIDE_AVAILABILITY_WINDOW"

var (
	log                      = logging.Logger("share/availability")
	overrideLogOnce          sync.Once
	ErrOutsideSamplingWindow = errors.New("timestamp outside sampling window")
)

// ResolveWindow returns window or the CELESTIA_OVERRIDE_AVAILABILITY_WINDOW value
// when that env var is set to a valid duration.
func ResolveWindow(window time.Duration) time.Duration {
	durationRaw, ok := os.LookupEnv(overrideWindowEnv)
	if !ok {
		return window
	}
	duration, err := time.ParseDuration(durationRaw)
	if err != nil {
		return window
	}
	overrideLogOnce.Do(func() {
		log.Infow("availability window overridden by", "env", overrideWindowEnv, "window", duration)
	})
	return duration
}

// IsWithinWindow checks whether the given timestamp is within the
// given AvailabilityWindow. If the window is disabled (0), it returns true for
// every timestamp.
func IsWithinWindow(t time.Time, window time.Duration) bool {
	window = ResolveWindow(window)
	if window == time.Duration(0) {
		return true
	}
	return time.Since(t) <= window
}
