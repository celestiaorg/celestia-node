package cmd

import (
	"context"
	"fmt"

	"github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/node"
)

var (
	p2pMutualFlag = "p2p.mutual"
)

// P2PFlags gives a set of p2p flags.
func P2PFlags() *flag.FlagSet {
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

// ParseP2PFlags parses P2P flags from the given cmd and applies values to Env.
func ParseP2PFlags(ctx context.Context, cmd *cobra.Command) (context.Context, error) {
	mutualPeers, err := cmd.Flags().GetStringSlice(p2pMutualFlag)
	if err != nil {
		return ctx, err
	}

	for _, peer := range mutualPeers {
		_, err := multiaddr.NewMultiaddr(peer)
		if err != nil {
			return ctx, fmt.Errorf("cmd: while parsing '%s': %w", p2pMutualFlag, err)
		}
	}

	if len(mutualPeers) != 0 {
		ctx = WithNodeOptions(ctx, node.WithMutualPeers(mutualPeers))
	}
	return ctx, nil
}
