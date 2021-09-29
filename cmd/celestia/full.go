package main

import (
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/node"
)

func init() {
	const repoName = "repository"
	fullCmd.AddCommand(cmd.Init(repoName, node.Full))
	fullCmd.PersistentFlags().StringP(repoName,
		"r",
		"~/.celestia-full",
		"The root/home directory of your Celestial Full Node",
	)
}

var fullCmd = &cobra.Command{
	Use: "full [subcommand]",
}