package main

import (
	"github.com/spf13/cobra"

	cmdnode "github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/node"
	"github.com/celestiaorg/celestia-node/node/fxutil"
)

// NOTE: We should always ensure that the added Flags below are parsed somewhere, like in the PersistenPreRun func on
// parent command.

// NewLightCommand creates a new light sub command
func NewLightCommand(extraComponents fxutil.Option) *cobra.Command {
	command := &cobra.Command{
		Use:   "light [subcommand]",
		Args:  cobra.NoArgs,
		Short: "Manage your Light node",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			env, err := cmdnode.GetEnv(cmd.Context())
			if err != nil {
				return err
			}
			env.SetNodeType(node.Light)

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

	command.AddCommand(
		cmdnode.Init(
			cmdnode.NodeFlags(node.Light),
			cmdnode.P2PFlags(),
			cmdnode.HeadersFlags(),
			cmdnode.MiscFlags(),
		),
		cmdnode.Start(
			extraComponents,
			cmdnode.NodeFlags(node.Light),
			cmdnode.P2PFlags(),
			cmdnode.HeadersFlags(),
			cmdnode.MiscFlags(),
		),
	)

	return command
}
