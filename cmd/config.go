package cmd

import (
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/nodebuilder"
)

func RemoveConfigCmd(fsets ...*flag.FlagSet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config-remove",
		Short: "Deletes the node's config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return nodebuilder.RemoveConfig(StorePath(ctx))
		},
	}

	for _, set := range fsets {
		cmd.Flags().AddFlagSet(set)
	}
	return cmd
}

func UpdateConfigCmd(fsets ...*flag.FlagSet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config-update",
		Short: "Updates the node's outdated config",
		Long: "Updates the node's outdated config with default values from newly-added fields. Check the config " +
			" afterwards to ensure all old custom values were preserved.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return nodebuilder.UpdateConfig(NodeType(ctx), StorePath(ctx))
		},
	}

	for _, set := range fsets {
		cmd.Flags().AddFlagSet(set)
	}
	return cmd
}
