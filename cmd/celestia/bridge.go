package main

import (
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/node"
)

func init() {
	bridgeCmd.AddCommand(
		cmd.Init(storeFlagName, node.Bridge),
		cmd.Start(storeFlagName, node.Bridge),
	)
	bridgeCmd.PersistentFlags().StringP(
		storeFlagName,
		storeFlagShort,
		"~/.celestia-bridge",
		"The root/home directory of your Celestial Bridge Node",
	)
}

var bridgeCmd = &cobra.Command{
	Use:  "bridge [subcommand]",
	Args: cobra.NoArgs,
}
