package share

import "errors"

// ErrInvalidFragmentKey indicates that a stored fragment key does not
// conform to the expected CanonicalKey layout.
var ErrInvalidFragmentKey = errors.New("cda: invalid fragment key")

