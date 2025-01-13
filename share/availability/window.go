package availability

import (
	"errors"
	"os"
	"time"
)

const (
	RequestWindow = 30 * 24 * time.Hour
	StorageWindow = RequestWindow + time.Hour
)

var ErrOutsideSamplingWindow = errors.New("timestamp outside sampling window")

// IsWithinWindow checks whether the given timestamp is within the
// given AvailabilityWindow. If the window is disabled (0), it returns true for
// every timestamp.
func IsWithinWindow(t time.Time, window time.Duration) bool {
	if windowOverride {
		window = windowOverrideDur
	}
	if window == time.Duration(0) {
		return true
	}
	return time.Since(t) <= window
}

// windowOverride is a flag that overrides the availability window. This flag is intended for
// testing purposes only.
var (
	windowOverride    bool
	windowOverrideDur time.Duration
)

func init() {
	durationRaw, ok := os.LookupEnv("CELESTIA_OVERRIDE_AVAILABILITY_WINDOW")
	if !ok {
		return
	}

	duration, err := time.ParseDuration(durationRaw)
	if err != nil {
		panic("failed to parse CELESTIA_OVERRIDE_AVAILABILITY_WINDOW: " + err.Error())
	}

	windowOverride = true
	windowOverrideDur = duration
}
