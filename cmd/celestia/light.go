//nolint:dupl
package main

import (
	"github.com/spf13/cobra"

	cmdnode "github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/node"
)

// NOTE: We should always ensure that the added Flags below are parsed somewhere, like in the PersistentPreRun func on
// parent command.

func init() {
	lightCmd.AddCommand(
		cmdnode.Init(
			cmdnode.NodeFlags(node.Light),
			cmdnode.P2PFlags(),
			cmdnode.HeadersFlags(),
			cmdnode.MiscFlags(),
			// NOTE: for now, state-related queries can only be accessed
			// over an RPC connection with a celestia-core node.
			cmdnode.CoreFlags(),
			cmdnode.RPCFlags(),
			cmdnode.KeyFlags(),
		),
		cmdnode.Start(
			cmdnode.NodeFlags(node.Light),
			cmdnode.P2PFlags(),
			cmdnode.HeadersFlags(),
			cmdnode.MiscFlags(),
			// NOTE: for now, state-related queries can only be accessed
			// over an RPC connection with a celestia-core node.
			cmdnode.CoreFlags(),
			cmdnode.RPCFlags(),
			cmdnode.KeyFlags(),
		),
	)
}

var lightCmd = &cobra.Command{
	Use:   "light [subcommand]",
	Args:  cobra.NoArgs,
	Short: "Manage your Light node",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var (
			ctx = cmd.Context()
			err error
		)

		ctx = cmdnode.WithNodeType(ctx, node.Light)

		ctx, err = cmdnode.ParseNodeFlags(ctx, cmd)
		if err != nil {
			return err
		}

		ctx, err = cmdnode.ParseP2PFlags(ctx, cmd)
		if err != nil {
			return err
		}

		ctx, err = cmdnode.ParseCoreFlags(ctx, cmd)
		if err != nil {
			return err
		}

		ctx, err = cmdnode.ParseHeadersFlags(ctx, cmd)
		if err != nil {
			return err
		}

		ctx, err = cmdnode.ParseMiscFlags(ctx, cmd)
		if err != nil {
			return err
		}

		ctx, err = cmdnode.ParseRPCFlags(ctx, cmd)
		if err != nil {
			return err
		}

		ctx = cmdnode.ParseKeyFlags(ctx, cmd)

		cmd.SetContext(ctx)
		return nil
	},
}
