package shrex

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

var ErrInvalidRequest = errors.New("server returned error indicating request was malformed")

var ErrInternalServer = errors.New("server encountered unexpected error")

// ErrUnsupportedProtocol is returned when the server does not support selected protocol.
var ErrUnsupportedProtocol = errors.New("unsupported protocol")

// ErrResourceExhausted is returned when the server resets the stream because it has hit
// a resource limit (stream count or memory). The peer is not misbehaving; it is temporarily
// overloaded. The request should be retried on a different peer after a backoff.
var ErrResourceExhausted = errors.New("server resource limit exceeded")
