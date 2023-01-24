package p2p

import (
	"errors"
)

// ErrUnavailable is returned when a peer doesn't have the requested data or doesn't have the
// capacity to serve it at the moment. It is used to signal that the peer couldn't serve the data
// successfully, but should be retried later.
var ErrUnavailable = errors.New("server cannot serve the requested data at this time")

// ErrInvalidResponse is returned when a peer returns an invalid response or caused an internal
// error. It is used to signal that the peer couldn't serve the data successfully, and should not be
// retried.
var ErrInvalidResponse = errors.New("server returned an invalid response or caused an internal error")
