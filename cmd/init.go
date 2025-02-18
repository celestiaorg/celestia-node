package cmd

import (
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

// Init constructs a CLI command to initialize Celestia Node of any type with the given flags.
func Init(tp node.Type, fsets ...*flag.FlagSet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialization for Celestia Node. Passed flags have persisted effect.",
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return PersistentPreRunEnv(cmd, tp, args)
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			return nodebuilder.Init(NodeConfig(ctx), StorePath(ctx), NodeType(ctx))
		},
	}
	for _, set := range fsets {
		cmd.Flags().AddFlagSet(set)
	}
	return cmd
}
