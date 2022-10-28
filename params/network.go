package params

import (
	"errors"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
)

// NOTE: Every time we add a new long-running network, it has to be added here.
const (
	// Arabica testnet. See: celestiaorg/networks.
	Arabica Network = "arabica-1"
	// Mamaki testnet. See: celestiaorg/networks.
	Mamaki Network = "mamaki"
	// Private can be used to set up any private network, including local testing setups.
	Private Network = "private"
	// BlockTime is a network block time.
	// TODO @renaynay @Wondertan (#790)
	BlockTime = time.Second * 30
)

// Network is a type definition for DA network run by Celestia Node.
type Network string

// Bootstrappers is a type definition for nodes that will be used as bootstrappers.
type Bootstrappers []peer.AddrInfo

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
	Arabica: {},
	Mamaki:  {},
	Private: {},
}

// ListProvidedNetworks provides a string listing all known long-standing networks for things like command hints.
func ListProvidedNetworks() string {
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
