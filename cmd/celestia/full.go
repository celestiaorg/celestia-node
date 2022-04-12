//nolint:dupl
package main

import (
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/spf13/cobra"

	cmdnode "github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/node"
)

// NOTE: We should always ensure that the added Flags below are parsed somewhere, like in the PersistentPreRun func on
// parent command.

func init() {
	fullKeyCmd := keys.Commands("$HOME/.celestia-full/keys")
	fullKeyCmd.Short = "Manage your full node account keys"
	fullKeyCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		_, err := cmdnode.GetEnv(cmd.Context())
		return err
	}

	fullCmd.AddCommand(
		cmdnode.Init(
			cmdnode.NodeFlags(node.Full),
			cmdnode.P2PFlags(),
			cmdnode.HeadersFlags(),
			cmdnode.MiscFlags(),
			// NOTE: for now, state-related queries can only be accessed
			// over an RPC connection with a celestia-core node.
			cmdnode.CoreFlags(),
			cmdnode.RPCFlags(),
		),
		cmdnode.Start(
			cmdnode.NodeFlags(node.Full),
			cmdnode.P2PFlags(),
			cmdnode.HeadersFlags(),
			cmdnode.MiscFlags(),
			// NOTE: for now, state-related queries can only be accessed
			// over an RPC connection with a celestia-core node.
			cmdnode.CoreFlags(),
			cmdnode.RPCFlags(),
		),
		fullKeyCmd,
	)
}

var fullCmd = &cobra.Command{
	Use:   "full [subcommand]",
	Args:  cobra.NoArgs,
	Short: "Manage your Full node",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		env, err := cmdnode.GetEnv(cmd.Context())
		if err != nil {
			return err
		}

		env.SetNodeType(node.Full)

		err = cmdnode.ParseNodeFlags(cmd, env)
		if err != nil {
			return err
		}

		err = cmdnode.ParseP2PFlags(cmd, env)
		if err != nil {
			return err
		}

		err = cmdnode.ParseCoreFlags(cmd, env)
		if err != nil {
			return err
		}

		err = cmdnode.ParseHeadersFlags(cmd, env)
		if err != nil {
			return err
		}

		err = cmdnode.ParseMiscFlags(cmd)
		if err != nil {
			return err
		}

		err = cmdnode.ParseRPCFlags(cmd, env)
		if err != nil {
			return err
		}

		return nil
	},
}
