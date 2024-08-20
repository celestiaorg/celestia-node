package p2p

import (
	"errors"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// NOTE: Every time we add a new long-running network, it has to be added here.
const (
	// DefaultNetwork is the default network of the current build.
	DefaultNetwork = Mainnet
	// Arabica testnet. See: celestiaorg/networks.
	Arabica Network = "arabica-11"
	// Mocha testnet. See: celestiaorg/networks.
	Mocha Network = "mocha-4"
	// Private can be used to set up any private network, including local testing setups.
	Private Network = "private"
	// Celestia mainnet. See: celestiaorg/networks.
	Mainnet Network = "celestia"
	// BlockTime is a network block time.
	// TODO @renaynay @Wondertan (#790)
	BlockTime = time.Second * 10
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
	Mainnet: {},
	Arabica: {},
	Mocha:   {},
	Private: {},
}

// networkAliases is a strict list of all known long-standing networks
// mapped from the string representation of their *alias* (rather than
// their actual value) to the Network.
var networkAliases = map[string]Network{
	"mainnet": Mainnet,
	"arabica": Arabica,
	"mocha":   Mocha,
	"private": Private,
}

// orderedNetworks is a list of all known networks in order of priority.
var orderedNetworks = []Network{Mainnet, Mocha, Arabica, Private}

// GetNetworks provides a list of all known networks in order of priority.
func GetNetworks() []Network {
	return append([]Network(nil), orderedNetworks...)
}

// listAvailableNetworks provides a string listing all known long-standing networks for things
// like CLI hints.
func listAvailableNetworks() string {
	var networks []string
	for _, net := range orderedNetworks {
		// "private" networks are configured via env vars, so skip
		if net != Private {
			networks = append(networks, net.String())
		}
	}

	return strings.Join(networks, ", ")
}

// addCustomNetwork adds a custom network to the list of known networks.
func addCustomNetwork(network Network) {
	networksList[network] = struct{}{}
	networkAliases[network.String()] = network
	orderedNetworks = append(orderedNetworks, network)
}
