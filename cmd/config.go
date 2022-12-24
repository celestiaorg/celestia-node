package cmd

import (
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

var ConfigCmd = &cobra.Command{
	Use:   "config [subcommand]",
	Args:  cobra.NoArgs,
	Short: "Manage config for your bridge node",
}

func Remove(fsets ...*flag.FlagSet) *cobra.Command {
	RemoveConfigCmd := &cobra.Command{
		Use:   "remove",
		Args:  cobra.NoArgs,
		Short: "Remove current config",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return parseStorePath(cmd, node.Bridge)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return nodebuilder.Remove(StorePath(ctx))
		},
	}
	for _, set := range fsets {
		RemoveConfigCmd.Flags().AddFlagSet(set)
	}

	return RemoveConfigCmd
}

func Reinit(fsets ...*flag.FlagSet) *cobra.Command {
	ReinitCmd := &cobra.Command{
		Use:   "reinit [config-path]",
		Args:  cobra.MinimumNArgs(1),
		Short: "Reinit config",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return parseStorePath(cmd, node.Bridge)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return nodebuilder.Reinit(NodeConfig(ctx), StorePath(ctx), args[0], NodeType(ctx))
		},
	}
	for _, set := range fsets {
		ReinitCmd.Flags().AddFlagSet(set)
	}

	return ReinitCmd
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
