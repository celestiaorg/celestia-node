package shrex

import (
	"errors"
)

// ErrNotFound is returned when a peer is unable to find the requested data or resource.
// It is used to signal that the peer couldn't serve the data successfully, and it's not
// available at the moment. The request may be retried later, but it's unlikely to succeed.
var ErrNotFound = errors.New("the requested data or resource could not be found")

var ErrRateLimited = errors.New("server is overloaded and rate limited the request")

// ErrInvalidResponse is returned when a peer returns an invalid response or caused an internal
// error. It is used to signal that the peer couldn't serve the data successfully, and should not be
// retried.
var ErrInvalidResponse = errors.New("server returned an invalid response or caused an internal error")

var ErrInvalidRequest = errors.New("server returned error indicating request was malformed")

var ErrInternalServer = errors.New("server encountered unexpected error")
