package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/gateway"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/pruner"
	"github.com/celestiaorg/celestia-node/nodebuilder/rpc"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

func NewBridge(options ...func(*cobra.Command, node.Type, []*pflag.FlagSet)) *cobra.Command {
	flags := []*pflag.FlagSet{
		NodeFlags(),
		p2p.Flags(),
		MiscFlags(),
		core.Flags(),
		rpc.Flags(),
		gateway.Flags(),
		state.Flags(),
		pruner.Flags(),
	}
	cmd := &cobra.Command{
		Use:   "bridge [subcommand]",
		Args:  cobra.NoArgs,
		Short: "Manage your Bridge node",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			ctx := WithNodeType(cmd.Context(), node.Bridge)
			cmd.SetContext(ctx)
		},
	}

	for _, option := range options {
		option(cmd, node.Bridge, flags)
	}
	return cmd
}

func NewLight(options ...func(*cobra.Command, node.Type, []*pflag.FlagSet)) *cobra.Command {
	flags := []*pflag.FlagSet{
		NodeFlags(),
		p2p.Flags(),
		header.Flags(),
		MiscFlags(),
		core.Flags(),
		rpc.Flags(),
		gateway.Flags(),
		state.Flags(),
		pruner.Flags(),
	}
	cmd := &cobra.Command{
		Use:   "light [subcommand]",
		Args:  cobra.NoArgs,
		Short: "Manage your Light node",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			ctx := WithNodeType(cmd.Context(), node.Light)
			cmd.SetContext(ctx)
		},
	}
	for _, option := range options {
		option(cmd, node.Light, flags)
	}
	return cmd
}

func NewFull(options ...func(*cobra.Command, node.Type, []*pflag.FlagSet)) *cobra.Command {
	flags := []*pflag.FlagSet{
		NodeFlags(),
		p2p.Flags(),
		header.Flags(),
		MiscFlags(),
		core.Flags(),
		rpc.Flags(),
		gateway.Flags(),
		state.Flags(),
		pruner.Flags(),
	}
	cmd := &cobra.Command{
		Use:   "full [subcommand]",
		Args:  cobra.NoArgs,
		Short: "Manage your Full node",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			ctx := WithNodeType(cmd.Context(), node.Full)
			cmd.SetContext(ctx)
		},
	}
	for _, option := range options {
		option(cmd, node.Full, flags)
	}
	return cmd
}
