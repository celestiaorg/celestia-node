//nolint:dupl
package main

import (
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/rpc"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"

	cmdnode "github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

// NOTE: We should always ensure that the added Flags below are parsed somewhere, like in the PersistentPreRun func on
// parent command.

func init() {
	fullCmd.AddCommand(
		cmdnode.Init(
			cmdnode.NodeFlags(),
			p2p.Flags(),
			header.Flags(),
			cmdnode.MiscFlags(),
			// NOTE: for now, state-related queries can only be accessed
			// over an RPC connection with a celestia-core node.
			core.Flags(),
			rpc.Flags(),
			state.Flags(),
		),
		cmdnode.Start(
			cmdnode.NodeFlags(),
			p2p.Flags(),
			header.Flags(),
			cmdnode.MiscFlags(),
			// NOTE: for now, state-related queries can only be accessed
			// over an RPC connection with a celestia-core node.
			core.Flags(),
			rpc.Flags(),
			state.Flags(),
		),
	)
}

var fullCmd = &cobra.Command{
	Use:   "full [subcommand]",
	Args:  cobra.NoArgs,
	Short: "Manage your Full node",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var (
			ctx = cmd.Context()
			err error
		)

		ctx = cmdnode.WithNodeType(ctx, node.Full)

		ctx, err = cmdnode.ParseNodeFlags(ctx, cmd)
		if err != nil {
			return err
		}

		cfg := cmdnode.NodeConfig(ctx)

		err = p2p.ParseFlags(cmd, &cfg.P2P)
		if err != nil {
			return err
		}

		err = core.ParseFlags(cmd, &cfg.Core)
		if err != nil {
			return err
		}

		err = header.ParseFlags(cmd, &cfg.Header)
		if err != nil {
			return err
		}

		ctx, err = cmdnode.ParseMiscFlags(ctx, cmd)
		if err != nil {
			return err
		}

		rpc.ParseFlags(cmd, &cfg.RPC)
		state.ParseFlags(cmd, &cfg.State)

		// set config
		ctx = cmdnode.WithNodeConfig(ctx, &cfg)
		cmd.SetContext(ctx)
		return nil
	},
}
