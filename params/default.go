package params

import "github.com/libp2p/go-libp2p-core/peer"

// defaultNetwork defines a default network for the Celestia Node.
var defaultNetwork = DevNet

// DefaultNetwork returns the network of the current build.
func DefaultNetwork() Network {
	return defaultNetwork
}

// DefaultGenesis reports a hash of a genesis block for the current network.
func DefaultGenesis() string {
	return genesisList[defaultNetwork] // network is guaranteed to be valid
}

// DefaultBootstrappersInfos returns address information of bootstrap peers for the node's current network.
func DefaultBootstrappersInfos() []peer.AddrInfo {
	infos, err := parseAddrInfos(DefaultBootstrappers())
	if err != nil {
		panic(err)
	}
	return infos
}

// DefaultBootstrappers returns multiaddresses of bootstrap peers for the node's current network.
func DefaultBootstrappers() []string {
	return bootstrapList[defaultNetwork] // network is guaranteed to be valid
}
