package cmd

import (
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/node"
)

// NOTE: We should always ensure that the added Flags below are parsed somewhere, like in the PersistentPreRun func on
// parent command.

func NewFullCommand(plugs []node.Plugin) *cobra.Command {
	command := &cobra.Command{
		Use:   "full [subcommand]",
		Args:  cobra.NoArgs,
		Short: "Manage your Full node",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			env, err := GetEnv(cmd.Context())
			if err != nil {
				return err
			}
			env.SetNodeType(node.Full)

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
			NodeFlags(node.Full),
			P2PFlags(),
			TrustedHashFlags(),
			HeadersFlags(),
			MiscFlags(),
		),
		Start(
			plugs,
			NodeFlags(node.Full),
			P2PFlags(),
			TrustedHashFlags(),
			HeadersFlags(),
			MiscFlags(),
		),
	)
	return command
}
