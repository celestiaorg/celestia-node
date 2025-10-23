package availability

import (
	"errors"
	"os"
	"time"
)

const (
	SamplingWindow = 7 * 24 * time.Hour // Ref: CIP-036
	RequestWindow  = SamplingWindow
	StorageWindow  = RequestWindow + time.Hour
)

var ErrOutsideSamplingWindow = errors.New("timestamp outside sampling window")

// IsWithinWindow checks whether the given timestamp is within the
// given AvailabilityWindow. If the window is disabled (0), it returns true for
// every timestamp.
func IsWithinWindow(t time.Time, window time.Duration) bool {
	// check for environment variable override
	if durationRaw, ok := os.LookupEnv("CELESTIA_OVERRIDE_AVAILABILITY_WINDOW"); ok {
		if duration, err := time.ParseDuration(durationRaw); err == nil {
			window = duration
		}
	}
	if window == time.Duration(0) {
		return true
	}
	return time.Since(t) <= window
}
