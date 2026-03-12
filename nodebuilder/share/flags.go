package share

import (
	"fmt"

	"github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var rdaBootstrapPeersFlag = "rda-bootstrap-peers"

// Flags gives a set of share/RDA flags.
func Flags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.StringSlice(
		rdaBootstrapPeersFlag,
		nil,
		`Comma-separated multiaddresses of RDA bootstrap peers for grid discovery.
Uses these peers to fetch row/column peer lists for fast subnet joining.
Can be specified multiple times. (Format: multiformats.io/multiaddr)`,
	)

	return flags
}

// ParseFlags parses Share flags from the given cmd and saves them to the passed config.
func ParseFlags(cmd *cobra.Command, cfg *Config) error {
	bootstrapPeers, err := cmd.Flags().GetStringSlice(rdaBootstrapPeersFlag)
	if err != nil {
		return err
	}

	for _, peer := range bootstrapPeers {
		_, err = multiaddr.NewMultiaddr(peer)
		if err != nil {
			return fmt.Errorf("cmd: while parsing '%s': %w", rdaBootstrapPeersFlag, err)
		}
	}

	if len(bootstrapPeers) != 0 {
		cfg.RDABootstrapPeers = bootstrapPeers
	}
	return nil
}
