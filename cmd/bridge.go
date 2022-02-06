package cmd

import (
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/node"
)

// NOTE: We should always ensure that the added Flags below are parsed somewhere, like in the PersistenPreRun func on
// parent command.

// NewBridgeCommand creates a new bridge sub command. Provided plugins are
// installed into celestia-node
func NewBridgeCommand(plugs []node.Plugin) *cobra.Command {
	command := &cobra.Command{
		Use:   "bridge [subcommand]",
		Args:  cobra.NoArgs,
		Short: "Manage your Bridge node",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			env, err := GetEnv(cmd.Context())
			if err != nil {
				return err
			}
			env.SetNodeType(node.Bridge)

			err = ParseNodeFlags(cmd, env)
			if err != nil {
				return err
			}

			err = ParseP2PFlags(cmd, env)
			if err != nil {
				return err
			}

			err = ParseCoreFlags(cmd, env)
			if err != nil {
				return err
			}

			err = ParseMiscFlags(cmd)
			if err != nil {
				return err
			}

			return nil
		},
	}

	command.AddCommand(
		Init(
			plugs,
			NodeFlags(node.Bridge),
			P2PFlags(),
			CoreFlags(),
			MiscFlags(),
		),
		Start(
			plugs,
			NodeFlags(node.Bridge),
			P2PFlags(),
			CoreFlags(),
			MiscFlags(),
		),
	)

	return command
}
