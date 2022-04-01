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
	if genesis, ok := os.LookupEnv("CELESTIA_PRIVATE_GENESIS"); ok {
		defaultNetwork = Private
		genesisList[Private] = strings.ToUpper(genesis)
	}
}
