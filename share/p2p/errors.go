package p2p

import (
	"errors"
)

// ErrNotFound is returned when a peer is unable to find the requested data or resource.
// It is used to signal that the peer couldn't serve the data successfully, and it's not
// available at the moment. The request may be retried later, but it's unlikely to succeed.
var ErrNotFound = errors.New("the requested data or resource could not be found")

// ErrInvalidResponse is returned when a peer returns an invalid response or caused an internal
// error. It is used to signal that the peer couldn't serve the data successfully, and should not be
// retried.
var ErrInvalidResponse = errors.New("server returned an invalid response or caused an internal error")
