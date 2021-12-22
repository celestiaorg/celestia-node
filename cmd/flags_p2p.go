package cmd

import (
	"fmt"

	"github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/node"
)

var (
	p2pMutualF = "p2p.mutual"
)

func P2PFlags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.StringSlice(
		p2pMutualF,
		nil,
		`Comma-separated multiaddresses of mutual peers to keep a prioritized connection with.
Such connection is immune to peer scoring slashing and connection manager trimming.
Peers must bidirectionally point to each other. (Format: multiformats.io/multiaddr)
`,
	)

	return flags
}

func ParseP2PFlags(cmd *cobra.Command, env *Env) error {
	mutualPeers, err := cmd.Flags().GetStringSlice(p2pMutualF)
	if err != nil {
		return err
	}

	for _, peer := range mutualPeers {
		_, err := multiaddr.NewMultiaddr(peer)
		if err != nil {
			return fmt.Errorf("cmd: while parsing '%s': %w", p2pMutualF, err)
		}
	}

	if len(mutualPeers) != 0 {
		env.addOption(node.WithMutualPeers(mutualPeers))
	}
	return nil
}
