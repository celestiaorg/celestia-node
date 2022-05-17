package params

import (
	"fmt"
	"os"
	"strings"
)

// defaultNetwork defines a default network for the Celestia Node.
var defaultNetwork = DevNet

// DefaultNetwork returns the network of the current build.
func DefaultNetwork() Network {
	return defaultNetwork
}

func init() {
	// check if a different network from the registry was specified
	if network, ok := os.LookupEnv("CELESTIA_NETWORK"); ok {
		if _, exists := networksList[Network(network)]; !exists {
			panic("unknown network specified")
		}
		defaultNetwork = Network(network)
		return
	}
	// check if private network option set
	if genesis, ok := os.LookupEnv("CELESTIA_PRIVATE_GENESIS"); ok {
		defaultNetwork = Private
		genesisList[Private] = strings.ToUpper(genesis)
	}
	// check if custom network option set
	if customNet, ok := os.LookupEnv("CELESTIA_CUSTOM"); ok {
		fmt.Print("\n\nWARNING: Celestia custom network specified. Only use this option if the node is " +
			"freshly created and initialized.\n**DO NOT** run a custom network over an already-existing node " +
			"store!\n\n")

		params := strings.Split(customNet, "=")
		// ensure both params are present
		if len(params) != 2 {
			panic("must provide CELESTIA_CUSTOM in this format: <network_ID>=<genesis hash>")
		}
		netID, genHash := params[0], params[1]

		defaultNetwork = Network(netID)
		// register new network and set as default for node to use
		networksList[defaultNetwork] = struct{}{}
		genesisList[defaultNetwork] = strings.ToUpper(genHash)
	}
	// check if custom bootstrappers were provided for a network
	if bootstrappers, ok := os.LookupEnv("CELESTIA_BOOTSTRAPPERS"); ok {
		params := strings.Split(bootstrappers, "=")
		// ensure both params are present
		if len(params) != 2 {
			panic("must provide CELESTIA_BOOTSTRAPPERS in this format: " +
				"<network_ID>=<boostrappers comma separated list>")
		}

		netID, list := params[0], params[1]
		bootstrapList[Network(netID)] = strings.Split(list, ",")
	}
}
