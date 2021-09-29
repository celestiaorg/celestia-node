package main

import (
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/node"
)

func init()  {
	const repoName = "repository"
	lightCmd.AddCommand(cmd.Init(repoName, node.Light))
	lightCmd.PersistentFlags().StringP(
		repoName,
		"r",
		"~/.celestia-light",
		"The root/home directory of your Celestial Light Node",
	)
}

var lightCmd = &cobra.Command{
	Use: "light [subcommand]",
	Args: cobra.NoArgs,
}
