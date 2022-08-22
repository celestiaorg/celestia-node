//nolint:dupl
package main

import (
	"github.com/spf13/cobra"

	cmdnode "github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/node/header"
	"github.com/celestiaorg/celestia-node/node/node"
	"github.com/celestiaorg/celestia-node/node/state"
)

// NOTE: We should always ensure that the added Flags below are parsed somewhere, like in the PersistentPreRun func on
// parent command.

func init() {
	fullCmd.AddCommand(
		cmdnode.Init(
			cmdnode.NodeFlags(node.Full),
			cmdnode.P2PFlags(),
			header.HeadersFlags(),
			cmdnode.MiscFlags(),
			// NOTE: for now, state-related queries can only be accessed
			// over an RPC connection with a celestia-core node.
			cmdnode.CoreFlags(),
			cmdnode.RPCFlags(),
			state.KeyFlags(),
		),
		cmdnode.Start(
			cmdnode.NodeFlags(node.Full),
			cmdnode.P2PFlags(),
			header.HeadersFlags(),
			cmdnode.MiscFlags(),
			// NOTE: for now, state-related queries can only be accessed
			// over an RPC connection with a celestia-core node.
			cmdnode.CoreFlags(),
			cmdnode.RPCFlags(),
			state.KeyFlags(),
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

		ctx, err = cmdnode.ParseP2PFlags(ctx, cmd)
		if err != nil {
			return err
		}

		ctx, err = cmdnode.ParseCoreFlags(ctx, cmd)
		if err != nil {
			return err
		}

		ctx, err = header.ParseHeadersFlags(ctx, cmd)
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

		opts := state.ParseKeyFlags(cmd)
		if len(opts) > 0 {
			ctx = cmdnode.WithNodeOptions(ctx, opts...)
		}

		cmd.SetContext(ctx)
		return nil
	},
}
