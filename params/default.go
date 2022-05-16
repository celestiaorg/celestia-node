package params

import (
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
	// check if private network option set
	if genesis, ok := os.LookupEnv("CELESTIA_PRIVATE_GENESIS"); ok {
		defaultNetwork = Private
		genesisList[Private] = strings.ToUpper(genesis)
	}
	// check if custom network option set
	if customNet, ok := os.LookupEnv("CELESTIA_CUSTOM_NETWORK"); ok {
		genesis, ok := os.LookupEnv("CELESTIA_CUSTOM_GENESIS")
		if !ok {
			panic("custom network specified without a custom genesis hash")
		}
		bootstrappers, ok := os.LookupEnv("CELESTIA_CUSTOM_BOOTSTRAPPERS")
		if !ok {
			panic("custom network specified without custom bootstrappers")
		}
		// set all params
		defaultNetwork = Network(customNet)
		networksList[defaultNetwork] = struct{}{}
		genesisList[defaultNetwork] = strings.ToUpper(genesis)
		bootstrapList[defaultNetwork] = strings.Split(bootstrappers, ",")
	}
}
