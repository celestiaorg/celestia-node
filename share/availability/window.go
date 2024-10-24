package availability

import "time"

type Window time.Duration

func (w Window) Duration() time.Duration {
	return time.Duration(w)
}

// IsWithinWindow checks whether the given timestamp is within the
// given AvailabilityWindow. If the window is disabled (0), it returns true for
// every timestamp.
func IsWithinWindow(t time.Time, window time.Duration) bool {
	if window == time.Duration(0) {
		return true
	}
	return time.Since(t) <= window
}
