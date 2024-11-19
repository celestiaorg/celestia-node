package share

import (
	"time"
)

// Window is a type alias for time.Duration used in nodebuilder
// to provide sampling windows to their relevant components
type Window time.Duration

func (w Window) Duration() time.Duration {
	return time.Duration(w)
}
