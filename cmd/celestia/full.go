package main

import (
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/node"
)

func init() {
	fullCmd.AddCommand(
		cmd.Init(repoFlagName, node.Full),
		cmd.Start(repoFlagName, node.Full),
	)
	fullCmd.PersistentFlags().StringP(repoFlagName,
		repoFlagShort,
		"~/.celestia-full",
		"The root/home directory of your Celestial Full Node",
	)
}

var fullCmd = &cobra.Command{
	Use:  "full [subcommand]",
	Args: cobra.NoArgs,
}
