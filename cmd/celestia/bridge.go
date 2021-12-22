package main

import (
	"github.com/spf13/cobra"

	cmdnode "github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/node"
)

func init() {
	bridgeCmd.AddCommand(
		cmdnode.Init(
			cmdnode.NodeFlags(node.Bridge),
			cmdnode.P2PFlags(),
			cmdnode.CoreFlags(),
			cmdnode.MiscFlags(),
		),
		cmdnode.Start(
			cmdnode.NodeFlags(node.Bridge),
			cmdnode.P2PFlags(),
			cmdnode.CoreFlags(),
			cmdnode.MiscFlags(),
		),
	)
}

var bridgeCmd = &cobra.Command{
	Use:  "bridge [subcommand]",
	Args: cobra.NoArgs,
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

		err = cmdnode.ParseMiscFlags(cmd)
		if err != nil {
			return err
		}

		return nil
	},
}
