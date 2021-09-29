package main

import (
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/node"
)

func init()  {
	lightCmd.AddCommand(lightCmd)
	lightCmd.Annotations[tp] = node.Light.String()
	lightCmd.PersistentFlags().String("repository", "~/.celestia-light", "The root/home directory for your Celestial Light Node")
}

var lightCmd = &cobra.Command{
	Use: "light [subcommand]",
}
