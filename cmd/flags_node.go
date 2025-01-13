package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

const (
	nodeStoreFlag  = "node.store"
	nodeConfigFlag = "node.config"
)

// NodeFlags gives a set of hardcoded Node package flags.
func NodeFlags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.String(
		nodeStoreFlag,
		"",
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
func ParseNodeFlags(ctx context.Context, cmd *cobra.Command, network p2p.Network) (context.Context, error) {
	store := cmd.Flag(nodeStoreFlag).Value.String()
	if store == "" {
		tp := NodeType(ctx)
		var err error
		store, err = nodebuilder.DefaultNodeStorePath(tp, network)
		if err != nil {
			return ctx, err
		}
	}
	ctx = WithStorePath(ctx, store)

	nodeConfig := cmd.Flag(nodeConfigFlag).Value.String()
	if nodeConfig != "" {
		// try to load config from given path
		cfg, err := nodebuilder.LoadConfig(nodeConfig)
		if err != nil {
			return ctx, fmt.Errorf("cmd: while parsing '%s': %w", nodeConfigFlag, err)
		}

		ctx = WithNodeConfig(ctx, cfg)
	} else {
		// check if config already exists at the store path and load it
		path := StorePath(ctx)
		expanded, err := homedir.Expand(filepath.Clean(path))
		if err != nil {
			return ctx, err
		}
		cfg, err := nodebuilder.LoadConfig(filepath.Join(expanded, "config.toml"))
		if err == nil {
			ctx = WithNodeConfig(ctx, cfg)
		}
	}
	return ctx, nil
}
