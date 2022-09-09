package params

import (
	"fmt"
	"os"
	"strings"
)

const (
	EnvCustomNetwork  = "CELESTIA_CUSTOM"
	EnvPrivateGenesis = "CELESTIA_PRIVATE_GENESIS"
)

// defaultNetwork defines a default network for the Celestia Node.
var defaultNetwork = Arabica

// DefaultNetwork returns the network of the current build.
func DefaultNetwork() Network {
	return defaultNetwork
}

func init() {
	// check if custom network option set
	// format: CELESTIA_CUSTOM=<netID>:<genesisHash>:<bootstrapPeerList>
	if custom, ok := os.LookupEnv(EnvCustomNetwork); ok {
		fmt.Print("\n\nWARNING: Celestia custom network specified. Only use this option if the node is " +
			"freshly created and initialized.\n**DO NOT** run a custom network over an already-existing node " +
			"store!\n\n")
		// ensure at least custom network is set
		params := strings.Split(custom, ":")
		if len(params) == 0 {
			panic("params: must provide at least <network_ID> to use a custom network")
		}
		netID := params[0]
		defaultNetwork = Network(netID)
		networksList[defaultNetwork] = struct{}{}
		// check if genesis hash provided and register it if exists
		if len(params) >= 2 {
			genHash := params[1]
			genesisList[defaultNetwork] = strings.ToUpper(genHash)
		}
		// check if bootstrappers were provided and register
		if len(params) == 3 {
			bootstrappers := params[2]
			// validate bootstrappers
			bs := strings.Split(bootstrappers, ",")
			_, err := parseAddrInfos(bs)
			if err != nil {
				println(fmt.Sprintf("params: env %s: contains invalid multiaddress", EnvCustomNetwork))
				panic(err)
			}
			bootstrapList[Network(netID)] = bs
		}
	}
	// check if private network option set
	if genesis, ok := os.LookupEnv(EnvPrivateGenesis); ok {
		defaultNetwork = Private
		genesisList[Private] = strings.ToUpper(genesis)
	}
}
