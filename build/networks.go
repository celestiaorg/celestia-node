package build

import "errors"

// DefaultNetwork sets a default network for Celestia Node.
var DefaultNetwork = DevNet

// NOTE: Everytime we add a new long-running network, it has to be added it here.
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
	if _, ok := networksList[n]; ok {
		return nil
	} else {
		return ErrInvalidNetwork
	}
}

// A strict list of networks.
var networksList = map[Network]struct{}{
	DevNet: {},
}
