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
	bridgeKeyCmd := keys.Commands("~/.celestia-bridge/keys")
	bridgeKeyCmd.Short = "Manage your bridge node account keys"
	bridgeKeyCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		_, err := cmdnode.GetEnv(cmd.Context())
		return err
	}

	bridgeCmd.AddCommand(
		cmdnode.Init(
			cmdnode.NodeFlags(node.Bridge),
			cmdnode.P2PFlags(),
			cmdnode.CoreFlags(),
			cmdnode.TrustedHashFlags(),
			cmdnode.MiscFlags(),
		),
		cmdnode.Start(
			cmdnode.NodeFlags(node.Bridge),
			cmdnode.P2PFlags(),
			cmdnode.CoreFlags(),
			cmdnode.TrustedHashFlags(),
			cmdnode.MiscFlags(),
		),
		bridgeKeyCmd,
	)
}

var bridgeCmd = &cobra.Command{
	Use:   "bridge [subcommand]",
	Args:  cobra.NoArgs,
	Short: "Manage your Bridge node",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		env, err := cmdnode.GetEnv(cmd.Context())
		if err != nil {
			return err
		}

		env.SetNodeType(node.Bridge)

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

		err = cmdnode.ParseTrustedHashFlags(cmd, env)
		if err != nil {
			return err
		}

		err = cmdnode.ParseMiscFlags(cmd)
		if err != nil {
			return err
		}

		return nil
	},
}
