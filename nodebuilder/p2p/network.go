package p2p

import (
	"errors"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// NOTE: Every time we add a new long-running network, it has to be added here.
const (
	// DefaultNetwork is the default network of the current build.
	DefaultNetwork = Mocha
	// Arabica testnet. See: celestiaorg/networks.
	Arabica Network = "arabica-10"
	// Mocha testnet. See: celestiaorg/networks.
	Mocha Network = "mocha-3"
	// BlockspaceRace testnet. See: https://docs.celestia.org/nodes/blockspace-race/.
	BlockspaceRace Network = "blockspacerace-0"
	// Private can be used to set up any private network, including local testing setups.
	Private Network = "private"
	// BlockTime is a network block time.
	// TODO @renaynay @Wondertan (#790)
	BlockTime = time.Second * 15
)

// Network is a type definition for DA network run by Celestia Node.
type Network string

// Bootstrappers is a type definition for nodes that will be used as bootstrappers.
type Bootstrappers []peer.AddrInfo

// ErrInvalidNetwork is thrown when unknown network is used.
var ErrInvalidNetwork = errors.New("params: invalid network")

// Validate the network.
func (n Network) Validate() (Network, error) {
	// return actual network if alias was provided
	if net, ok := networkAliases[string(n)]; ok {
		return net, nil
	}
	if _, ok := networksList[n]; !ok {
		return "", ErrInvalidNetwork
	}
	return n, nil
}

// String returns string representation of the Network.
func (n Network) String() string {
	return string(n)
}

// networksList is a strict list of all known long-standing networks.
var networksList = map[Network]struct{}{
	Arabica:        {},
	Mocha:          {},
	BlockspaceRace: {},
	Private:        {},
}

// networkAliases is a strict list of all known long-standing networks
// mapped from the string representation of their *alias* (rather than
// their actual value) to the Network.
var networkAliases = map[string]Network{
	"arabica":        Arabica,
	"mocha":          Mocha,
	"blockspacerace": BlockspaceRace,
	"private":        Private,
}

// listProvidedNetworks provides a string listing all known long-standing networks for things like
// command hints.
func listProvidedNetworks() string {
	var networks string
	for net := range networksList {
		// "private" network isn't really a choosable option, so skip
		if net != Private {
			networks += string(net) + ", "
		}
	}
	// chop off trailing ", "
	return networks[:len(networks)-2]
}
