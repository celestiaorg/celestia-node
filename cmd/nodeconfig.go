package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

func Remove(nodeType node.Type, fsets ...*pflag.FlagSet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conf-remove",
		Args:  cobra.NoArgs,
		Short: "Remove current config",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return parseStorePath(cmd, nodeType)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return nodebuilder.Remove(StorePath(ctx))
		},
	}
	for _, set := range fsets {
		cmd.Flags().AddFlagSet(set)
	}

	return cmd
}

func Reinit(nodeType node.Type, fsets ...*pflag.FlagSet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conf-reinit [config-path]",
		Args:  cobra.MinimumNArgs(1),
		Short: "Reinit config",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return parseStorePath(cmd, nodeType)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return nodebuilder.Reinit(StorePath(ctx), args[0], NodeType(ctx))
		},
	}
	for _, set := range fsets {
		cmd.Flags().AddFlagSet(set)
	}

	return cmd
}

func parseStorePath(cmd *cobra.Command, nodeType node.Type) error {
	var (
		ctx = cmd.Context()
		err error
	)

	ctx = WithNodeType(ctx, nodeType)

	parsedNetwork, err := p2p.ParseNetwork(cmd)
	if err != nil {
		return err
	}
	ctx = WithNetwork(ctx, parsedNetwork)

	ctx, err = ParseNodeFlags(ctx, cmd, Network(ctx))
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)
	return nil
}
