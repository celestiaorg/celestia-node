package main

import (
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/rpc"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"

	cmdnode "github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

// NOTE: We should always ensure that the added Flags below are parsed somewhere, like in the PersistentPreRun func on
// parent command.

func init() {
	bridgeCmd.AddCommand(
		cmdnode.Init(
			cmdnode.NodeFlags(),
			p2p.Flags(),
			core.Flags(),
			cmdnode.MiscFlags(),
			rpc.Flags(),
			state.Flags(),
		),
		cmdnode.Start(
			cmdnode.NodeFlags(),
			p2p.Flags(),
			core.Flags(),
			cmdnode.MiscFlags(),
			rpc.Flags(),
			state.Flags(),
		),
	)
}

var bridgeCmd = &cobra.Command{
	Use:   "bridge [subcommand]",
	Args:  cobra.NoArgs,
	Short: "Manage your Bridge node",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmdnode.WithNodeType(cmd.Context(), node.Bridge)

		ctx, cfg, err := parseBaseFlags(ctx, cmd)
		if err != nil {
			return err
		}

		// set config
		ctx = cmdnode.WithNodeConfig(ctx, cfg)
		cmd.SetContext(ctx)
		return nil
	},
}
