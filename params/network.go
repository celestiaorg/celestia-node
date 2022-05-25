package params

import (
	"errors"
)

// NOTE: Every time we add a new long-running network, it has to be added here.
const (
	// Mamaki testnet. See: celestiaorg/networks.
	Mamaki Network = "mamaki"
	// Private can be used to set up any private network, including local testing setups.
	// Use CELESTIA_PRIVATE_GENESIS env var to enable Private by specifying its genesis block hash.
	Private Network = "private"
)

// Network is a type definition for DA network run by Celestia Node.
type Network string

// ErrInvalidNetwork is thrown when unknown network is used.
var ErrInvalidNetwork = errors.New("params: invalid network")

// Validate the network.
func (n Network) Validate() error {
	if _, ok := networksList[n]; !ok {
		return ErrInvalidNetwork
	}
	return nil
}

// networksList is a strict list of all known long-standing networks.
var networksList = map[Network]struct{}{
	Mamaki:  {},
	Private: {},
}
