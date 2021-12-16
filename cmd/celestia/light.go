package main

import (
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/node"
)

func init() {
	lightCmd.AddCommand(
		cmd.Init(storeFlagName, node.Light),
		cmd.Start(storeFlagName, node.Light),
	)
	lightCmd.PersistentFlags().StringP(
		storeFlagName,
		storeFlagShort,
		"~/.celestia-light",
		"The root/home directory of your Celestial Light Node",
	)
}

var lightCmd = &cobra.Command{
	Use:  "light [subcommand]",
	Args: cobra.NoArgs,
}
