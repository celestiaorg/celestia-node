package main

import (
	"os"

	"github.com/spf13/cobra"
)

const (
	storeFlagName  = "store.path"
	storeFlagShort = "s"
)

func init() {
	rootCmd.AddCommand(
		bridgeCmd,
		lightCmd,
		versionCmd,
	)
}

func main() {
	err := run()
	if err != nil {
		os.Exit(1)
	}
}

func run() error {
	return rootCmd.Execute()
}

var rootCmd = &cobra.Command{
	Use: "celestia [  bridge  ||  light  ] [subcommand]",
	Short: `
	  / ____/__  / /__  _____/ /_(_)___ _
	 / /   / _ \/ / _ \/ ___/ __/ / __  /
	/ /___/  __/ /  __(__  ) /_/ / /_/ /
	\____/\___/_/\___/____/\__/_/\__,_/
	`,
	Args: cobra.NoArgs,
}
