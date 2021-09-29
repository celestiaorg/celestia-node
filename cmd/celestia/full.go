package main

import (
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/node"
)

func init() {
	fullCmd.AddCommand(initCmd)
	fullCmd.Annotations[tp] = node.Full.String()
	fullCmd.PersistentFlags().StringP("repository", "r", "~/.celestia-full", "The root/home directory for your Celestial Full Node")
}

var fullCmd = &cobra.Command{
	Use: "full [subcommand]",
}