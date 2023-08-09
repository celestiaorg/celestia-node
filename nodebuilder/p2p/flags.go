package p2p

import (
	"fmt"
	"os"
	"strings"

	"github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

// EnvCustomNetwork is the environment variable name used for setting a custom network.
// Note that setting a custom network overrides network/chainID validation rules - so
// the custom network name MUST match the chainID used by the core consensus network.
// Examples of valid custom network names: `custom-network`, `customnetwork`, `custom-celestia-network`.
const EnvCustomNetwork = "CELESTIA_CUSTOM"

const (
	networkFlag = "p2p.network"
	mutualFlag  = "p2p.mutual"
)

// Flags gives a set of p2p flags.
func Flags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.StringSlice(
		mutualFlag,
		nil,
		`Comma-separated multiaddresses of mutual peers to keep a prioritized connection with.
Such connection is immune to peer scoring slashing and connection module trimming.
Peers must bidirectionally point to each other. (Format: multiformats.io/multiaddr)
`,
	)
	flags.String(
		networkFlag,
		"",
		"The name of the network to connect to, e.g. "+
			listProvidedNetworks()+
			". Must be passed on both init and start to take effect.",
	)

	return flags
}

// ParseFlags parses P2P flags from the given cmd and saves them to the passed config.
func ParseFlags(
	cmd *cobra.Command,
	cfg *Config,
) error {
	mutualPeers, err := cmd.Flags().GetStringSlice(mutualFlag)
	if err != nil {
		return err
	}

	for _, peer := range mutualPeers {
		_, err = multiaddr.NewMultiaddr(peer)
		if err != nil {
			return fmt.Errorf("cmd: while parsing '%s': %w", mutualFlag, err)
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
	parsed := cmd.Flag(networkFlag).Value.String()
	// no network set through the flags, so check if there is an override in the env
	if parsed == "" {
		envNetwork, err := parseNetworkFromEnv()
		// no network found in env, so use the default network
		if envNetwork == "" {
			return DefaultNetwork, err
		}
		return envNetwork, err
	}
	// check if user provided the actual network value
	// or an alias
	if net, err := Network(parsed).Validate(); err == nil {
		return net, nil
	}
	return "", fmt.Errorf("invalid network specified: %s", parsed)
}

// parseNetworkFromEnv tries to parse the network from the environment.
// If no network is set, it returns an empty string.
func parseNetworkFromEnv() (Network, error) {
	var network Network
	// check if custom network option set
	// format: CELESTIA_CUSTOM=<netID>:<genesisHash>:<bootstrapPeerList>
	if custom, ok := os.LookupEnv(EnvCustomNetwork); ok {
		fmt.Print("\n\nWARNING: Celestia custom network specified. Only use this option if the node is " +
			"freshly created and initialized.\n**DO NOT** run a custom network over an already-existing node " +
			"store!\n\n")
		// ensure at least custom network is set
		params := strings.Split(custom, ":")
		if len(params) == 0 {
			return network, fmt.Errorf("params: must provide at least <network_ID> to use a custom network")
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
