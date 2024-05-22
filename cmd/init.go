package cmd

import (
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

// Init constructs a CLI command to initialize Celestia Node of any type with the given flags.
func Init(fsets ...*flag.FlagSet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialization for Celestia Node. Passed flags have persisted effect.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			node, err := NewRunner(cmd.Context())
			if err != nil {
				return err
			}

			return node.Init(cmd.Context())
		},
	}
	for _, set := range fsets {
		cmd.Flags().AddFlagSet(set)
	}
	return cmd
}
