package cmd

import (
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/node"
)

// NOTE: We should always ensure that the added Flags below are parsed somewhere, like in the PersistenPreRun func on
// parent command.

// NewLightCommand creates a new light sub command
func NewLightCommand(plugs []node.Plugin) *cobra.Command {
	command := &cobra.Command{
		Use:   "light [subcommand]",
		Args:  cobra.NoArgs,
		Short: "Manage your Light node",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			env, err := GetEnv(cmd.Context())
			if err != nil {
				return err
			}
			env.SetNodeType(node.Light)

			err = ParseNodeFlags(cmd, env)
			if err != nil {
				return err
			}

			err = ParseP2PFlags(cmd, env)
			if err != nil {
				return err
			}

			err = ParseHeadersFlags(cmd, env)
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
			NodeFlags(node.Light),
			P2PFlags(),
			HeadersFlags(),
			MiscFlags(),
		),
		Start(
			plugs,
			NodeFlags(node.Light),
			P2PFlags(),
			HeadersFlags(),
			MiscFlags(),
		),
	)

	return command
}
