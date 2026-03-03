package share

import "errors"

var (
	// RDA-related errors
	ErrSubnetNotInitialized = errors.New("subnet not initialized")
	ErrInvalidGridPosition  = errors.New("invalid grid position")
	ErrNoPeersAvailable     = errors.New("no peers available for communication")
	ErrCommunicationDenied  = errors.New("communication denied by filter policy")
	ErrRDANotStarted        = errors.New("RDA service not started")
)
