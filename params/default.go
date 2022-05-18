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
	// check if custom network option set
	if custom, ok := os.LookupEnv("CELESTIA_CUSTOM"); ok {
		fmt.Print("\n\nWARNING: Celestia custom network specified. Only use this option if the node is " +
			"freshly created and initialized.\n**DO NOT** run a custom network over an already-existing node " +
			"store!\n\n")
		// ensure all three params are present
		params := strings.Split(custom, ":")
		if len(params) != 3 {
			panic("must provide CELESTIA_CUSTOM in this format: " +
				"<network_ID>:<genesis hash>:<comma-separated list of bootstrappers>")
		}
		netID, genHash, bootstrappers := params[0], params[1], params[2]
		// validate bootstrappers
		bs := strings.Split(bootstrappers, ",")
		_, err := parseAddrInfos(bs)
		if err != nil {
			println("env CELESTIA_CUSTOM: contains invalid multiaddress")
			panic(err)
		}
		bootstrapList[Network(netID)] = bs
		// set custom network as default network for node to use
		defaultNetwork = Network(netID)
		// register network and genesis hash
		networksList[defaultNetwork] = struct{}{}
		genesisList[defaultNetwork] = strings.ToUpper(genHash)
	}
	// check if private network option set
	if genesis, ok := os.LookupEnv("CELESTIA_PRIVATE_GENESIS"); ok {
		defaultNetwork = Private
		genesisList[Private] = strings.ToUpper(genesis)
	}
}
