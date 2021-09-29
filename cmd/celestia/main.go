package main

import (
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/node"
)

func init() {
	rootCmd.AddCommand(fullCmd)
	rootCmd.AddCommand(lightCmd)
}

func main() {
 	err := run()
 	if err != nil {
		return
	}
}

func run() error {
	return rootCmd.Execute()
}

var rootCmd = &cobra.Command{
	Use: "celestia [subcommand]",
}

const tp = "type"

var initCmd = &cobra.Command{
	RunE: func(cmd *cobra.Command, args []string) error {
		tp := node.ParseType(cmd.Annotations["type"])
		path := cmd.Flag("repository").Value.String()
		node.Init(path, )
	},
}

