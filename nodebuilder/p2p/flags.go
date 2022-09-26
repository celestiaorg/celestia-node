package p2p

import (
	"fmt"
	"os"
	"strings"

	"github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

const (
	// DefaultNetwork is the default network of the current build.
	DefaultNetwork   = Arabica
	EnvCustomNetwork = "CELESTIA_CUSTOM"
	nodeNetworkFlag  = "node.network"
	p2pMutualFlag    = "p2p.mutual"
)

// Flags gives a set of p2p flags.
func Flags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.StringSlice(
		p2pMutualFlag,
		nil,
		`Comma-separated multiaddresses of mutual peers to keep a prioritized connection with.
Such connection is immune to peer scoring slashing and connection manager trimming.
Peers must bidirectionally point to each other. (Format: multiformats.io/multiaddr)
`,
	)
	flags.String(
		nodeNetworkFlag,
		"",
		"The name of the network to connect to, e.g. "+listProvidedNetworks(),
	)

	return flags
}

// ParseFlags parses P2P flags from the given cmd and saves them to the passed config.
func ParseFlags(
	cmd *cobra.Command,
	cfg *Config,
) error {
	mutualPeers, err := cmd.Flags().GetStringSlice(p2pMutualFlag)
	if err != nil {
		return err
	}

	for _, peer := range mutualPeers {
		_, err = multiaddr.NewMultiaddr(peer)
		if err != nil {
			return fmt.Errorf("cmd: while parsing '%s': %w", p2pMutualFlag, err)
		}
	}

	if len(mutualPeers) != 0 {
		cfg.MutualPeers = mutualPeers
	}
	return nil
}

// ParseNetwork tries to parse the network from the flags and environment,
// and returns either the parsed network or the build's default network
func ParseNetwork(cmd *cobra.Command) (Network, error) {
	parsedNetwork := cmd.Flag(nodeNetworkFlag).Value.String()
	// no network set through the flags, so check if there is an override in the env
	if parsedNetwork == "" {
		return ParseNetworkFromEnv()
	}
	return Network(parsedNetwork), nil
}

// ParseNetworkFromEnv tries to parse the network from the environment,
// and returns either the parsed network or the build's default network
func ParseNetworkFromEnv() (Network, error) {
	network := DefaultNetwork
	// check if custom network option set
	// format: CELESTIA_CUSTOM=<netID>:<genesisHash>:<bootstrapPeerList>
	if custom, ok := os.LookupEnv(EnvCustomNetwork); ok {
		fmt.Print("\n\nWARNING: Celestia custom network specified. Only use this option if the node is " +
			"freshly created and initialized.\n**DO NOT** run a custom network over an already-existing node " +
			"store!\n\n")
		// ensure at least custom network is set
		params := strings.Split(custom, ":")
		if len(params) == 0 {
			return DefaultNetwork, fmt.Errorf("params: must provide at least <network_ID> to use a custom network")
		}
		netID := params[0]
		network = Network(netID)
		networksList[network] = struct{}{}
		// check if genesis hash provided and register it if exists
		if len(params) >= 2 {
			genHash := params[1]
			genesisList[network] = strings.ToUpper(genHash)
		}
		// check if bootstrappers were provided and register
		if len(params) == 3 {
			bootstrappers := params[2]
			// validate bootstrappers
			bs := strings.Split(bootstrappers, ",")
			_, err := parseAddrInfos(bs)
			if err != nil {
				return DefaultNetwork, fmt.Errorf("params: env %s: contains invalid multiaddress", EnvCustomNetwork)
			}
			bootstrapList[Network(netID)] = bs
		}
	}
	return network, nil
}
