package p2p

import (
	"fmt"

	"github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var (
	p2pMutualFlag = "p2p.mutual"
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
