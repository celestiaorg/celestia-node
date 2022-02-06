package cmd

import (
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/node"
)

// Init constructs a CLI command to initialize Celestia Node of any type with the given flags.
func Init(plugs []node.Plugin, fsets ...*flag.FlagSet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialization for Celestia Node. Passed flags have persisted effect.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := GetEnv(cmd.Context())
			if err != nil {
				return err
			}

			env.opts = append(env.opts, node.WithPlugins(plugs...))

			return node.Init(env.StorePath, env.NodeType, env.Options()...)
		},
	}
	for _, set := range fsets {
		cmd.Flags().AddFlagSet(set)
	}
	return cmd
}
