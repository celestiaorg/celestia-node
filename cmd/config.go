package cmd

import (
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/gateway"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/rpc"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

func init() {
	ConfigCmd.AddCommand(
		Init(
			NodeFlags(),
			p2p.Flags(),
			core.Flags(),
			MiscFlags(),
			rpc.Flags(),
			gateway.Flags(),
			state.Flags(),
		),
		Remove(NodeFlags(),
			p2p.Flags(),
			core.Flags(),
			MiscFlags(),
			rpc.Flags(),
			gateway.Flags(),
			state.Flags()),
		Reinit(
			NodeFlags(),
			p2p.Flags(),
			core.Flags(),
			MiscFlags(),
			rpc.Flags(),
			gateway.Flags(),
			state.Flags(),
		),
	)
}

var ConfigCmd = &cobra.Command{
	Use:   "config [subcommand]",
	Args:  cobra.NoArgs,
	Short: "Manage your Bridge node",
}

func Remove(fsets ...*flag.FlagSet) *cobra.Command {
	RemoveConfigCmd := &cobra.Command{
		Use:   "remove",
		Args:  cobra.NoArgs,
		Short: "Remove current config",
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
