package main

import (
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/node"
)

func init() {
	bridgeCmd.AddCommand(
		cmd.Init(repoFlagName, node.Bridge),
		cmd.Start(repoFlagName, node.Bridge),
	)
	bridgeCmd.PersistentFlags().StringP(
		repoFlagName,
		repoFlagShort,
		"~/.celestia-bridge",
		"The root/home directory of your Celestial Bridge Node",
	)
}

var bridgeCmd = &cobra.Command{
	Use:  "bridge [subcommand]",
	Args: cobra.NoArgs,
}
