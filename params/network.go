package params

import "errors"

// NOTE: Every time we add a new long-running network, it has to be added here.
const (
	// DevNet or devnet-2 according to celestiaorg/networks
	DevNet Network = "devnet-2"
)

// Network is a type definition for DA network run by Celestia Node.
type Network string

// ErrInvalidNetwork is thrown when unknown network is used.
var ErrInvalidNetwork = errors.New("build: invalid network")

// Validate the network.
func (n Network) Validate() error {
	if _, ok := networksList[n]; !ok {
		return ErrInvalidNetwork
	}
	return nil
}

// networksList is a strict list of all known long-standing networks.
var networksList = map[Network]struct{}{
	DevNet: {},
}
