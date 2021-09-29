package main

import (
	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(fullCmd, lightCmd)
}

func main() {
 	err := run()
 	if err != nil {
		return
	}
}

func run() error {
	logging.SetAllLoggers(logging.LevelInfo)
	return rootCmd.Execute()
}

var rootCmd = &cobra.Command{
	Use: "celestia [subcommand]",
}
