package main

import (
	"github.com/spf13/cobra"

	cmdnode "github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/node"
)

// NOTE: We should always ensure that the added Flags below are parsed somewhere, like in the PersistentPreRun func on
// parent command.

func init() {
	fullCmd.AddCommand(
		cmdnode.Init(
			cmdnode.NodeFlags(node.Full),
			cmdnode.P2PFlags(),
			cmdnode.HeadersFlags(),
			cmdnode.MiscFlags(),
		),
		cmdnode.Start(
			cmdnode.NodeFlags(node.Full),
			cmdnode.P2PFlags(),
			cmdnode.HeadersFlags(),
			cmdnode.MiscFlags(),
		),
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

		err = cmdnode.ParseHeadersFlags(cmd, env)
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
