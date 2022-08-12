package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/node"
)

var (
	nodeStoreFlag  = "node.store"
	nodeConfigFlag = "node.config"
)

// NodeFlags gives a set of hardcoded Node package flags.
func NodeFlags(tp node.Type) *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.String(
		nodeStoreFlag,
		fmt.Sprintf("~/.celestia-%s", strings.ToLower(tp.String())),
		"The path to root/home directory of your Celestia Node Store",
	)
	flags.String(
		nodeConfigFlag,
		"",
		"Path to a customized node config TOML file",
	)

	return flags
}

// ParseNodeFlags parses Node flags from the given cmd and applies values to Env.
func ParseNodeFlags(ctx context.Context, cmd *cobra.Command) (context.Context, error) {
	ctx = WithStorePath(ctx, cmd.Flag(nodeStoreFlag).Value.String())

	nodeConfig := cmd.Flag(nodeConfigFlag).Value.String()
	if nodeConfig != "" {
		cfg, err := node.LoadConfig(nodeConfig)
		if err != nil {
			return ctx, fmt.Errorf("cmd: while parsing '%s': %w", nodeConfigFlag, err)
		}

		ctx = WithNodeOptions(ctx, node.WithConfig(cfg))
	}

	return ctx, nil
}
