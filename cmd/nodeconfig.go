package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

func NewConfigCmd(flags []*pflag.FlagSet, nodeType node.Type) *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config [subcommand]",
		Args:  cobra.NoArgs,
		Short: "Manage config for node",
	}

	configCmd.AddCommand(
		Init(flags...),
		Remove(nodeType, NodeFlags(), p2p.Flags(), MiscFlags()),
		Reinit(nodeType, NodeFlags(), p2p.Flags(), MiscFlags()),
	)

	return configCmd
}

// Init constructs a CLI command to initialize Celestia Node of any type with the given flags.
func Init(fsets ...*pflag.FlagSet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialization for Celestia Node. Passed flags have persisted effect.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			return nodebuilder.Init(NodeConfig(ctx), StorePath(ctx), NodeType(ctx))
		},
	}
	for _, set := range fsets {
		cmd.Flags().AddFlagSet(set)
	}

	return cmd
}

func Remove(nodeType node.Type, fsets ...*pflag.FlagSet) *cobra.Command {
	RemoveConfigCmd := &cobra.Command{
		Use:   "remove",
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
		RemoveConfigCmd.Flags().AddFlagSet(set)
	}

	return RemoveConfigCmd
}

func Reinit(nodeType node.Type, fsets ...*pflag.FlagSet) *cobra.Command {
	ReinitCmd := &cobra.Command{
		Use:   "reinit [config-path]",
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

	ctx, err = ParseMiscFlags(ctx, cmd)
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	return nil
}
