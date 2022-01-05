package cmd

import (
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

func ParseNodeFlags(cmd *cobra.Command, env *Env) error {
	env.storePath = cmd.Flag(nodeStoreFlag).Value.String()

	nodeConfig := cmd.Flag(nodeConfigFlag).Value.String()
	if nodeConfig != "" {
		cfg, err := node.LoadConfig(nodeConfig)
		if err != nil {
			return fmt.Errorf("cmd: while parsing '%s': %w", nodeConfigFlag, err)
		}

		env.addOption(node.WithConfig(cfg))
	}

	return nil
}
