package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/pruner"
	"github.com/celestiaorg/celestia-node/nodebuilder/rpc"
	"github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

func NewBridge(options ...func(*cobra.Command, []*pflag.FlagSet)) *cobra.Command {
	flags := []*pflag.FlagSet{
		NodeFlags(),
		p2p.Flags(),
		MiscFlags(),
		core.Flags(),
		rpc.Flags(),
		state.Flags(),
		pruner.Flags(),
		share.Flags(),
	}
	cmd := &cobra.Command{
		Use:   "bridge [subcommand]",
		Args:  cobra.NoArgs,
		Short: "Manage your Bridge node",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			ctx := WithNodeType(cmd.Context(), node.Bridge)
			cmd.SetContext(ctx)

			return nil
		},
	}
	for _, option := range options {
		option(cmd, flags)
	}
	return cmd
}

func NewLight(options ...func(*cobra.Command, []*pflag.FlagSet)) *cobra.Command {
	flags := []*pflag.FlagSet{
		NodeFlags(),
		p2p.Flags(),
		header.Flags(),
		MiscFlags(),
		core.Flags(),
		rpc.Flags(),
		state.Flags(),
		pruner.Flags(),
		share.Flags(),
	}
	cmd := &cobra.Command{
		Use:   "light [subcommand]",
		Args:  cobra.NoArgs,
		Short: "Manage your Light node",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			ctx := WithNodeType(cmd.Context(), node.Light)
			cmd.SetContext(ctx)
			return nil
		},
	}
	for _, option := range options {
		option(cmd, flags)
	}
	return cmd
}

func NewFull(options ...func(*cobra.Command, []*pflag.FlagSet)) *cobra.Command {
	flags := []*pflag.FlagSet{
		NodeFlags(),
		p2p.Flags(),
		header.Flags(),
		MiscFlags(),
		core.Flags(),
		rpc.Flags(),
		state.Flags(),
		pruner.Flags(),
		share.Flags(),
	}
	cmd := &cobra.Command{
		Use:   "full [subcommand]",
		Args:  cobra.NoArgs,
		Short: "Manage your Full node",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			log.Error(
				"DEPRECATION NOTICE: FULL NODE MODE WILL BE DEPRECATED SOON." +
					" NODE OPERATORS SHOULD CONSIDER RUNNING A BRIDGE NODE INSTEAD IF THEY REQUIRE FULL DATA STORAGE FUNCTIONALITY.",
			)
			ctx := WithNodeType(cmd.Context(), node.Full)
			cmd.SetContext(ctx)
			return nil
		},
	}
	for _, option := range options {
		option(cmd, flags)
	}
	return cmd
}
func NewPin(options ...func(*cobra.Command, []*pflag.FlagSet)) *cobra.Command {
	flags := []*pflag.FlagSet{
		NodeFlags(),
		p2p.Flags(),
		header.Flags(),
		MiscFlags(),
		core.Flags(),
		rpc.Flags(),
		state.Flags(),
		pruner.Flags(),
		share.Flags(),
	}
	cmd := &cobra.Command{
		Use:   "pin [subcommand]",
		Args:  cobra.NoArgs,
		Short: "Manage your Pin node (targeted namespace storage)",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			ctx := WithNodeType(cmd.Context(), node.Pin)
			cmd.SetContext(ctx)

			return nil
		},
	}
	for _, option := range options {
		option(cmd, flags)
	}
	return cmd
}
