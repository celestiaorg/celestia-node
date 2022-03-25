package params

import (
	"errors"
	"os"
)

// GetNetwork returns the network of the current build.
func GetNetwork() Network {
	return network
}

// DefaultNetwork sets a default network for Celestia Node.
var DefaultNetwork = DevNet

// NOTE: Every time we add a new long-running network, it has to be added here.
const (
	// DevNet or devnet-2 according to celestiaorg/networks
	DevNet Network = "devnet-2"
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
	DevNet:  {},
	Private: {},
}

// network is the currently used network within a build.
// It can be set with 'ldflags'.
var network Network

// init ensures `network` is always set and correct
func init() {
	if net, ok := os.LookupEnv("CELESTIA_NETWORK"); ok {
		network = Network(net)
		if err := network.Validate(); err != nil {
			println("env CELESTIA_NETWORK")
			panic(err)
		}

		return
	}

	if network == "" {
		network = DefaultNetwork
	}

	if err := network.Validate(); err != nil {
		panic(err)
	}
}
