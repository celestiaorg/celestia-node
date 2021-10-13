package main

import (
	"os"

	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(
		fullCmd,
		lightCmd,
		devCmd,
	)
}

func main() {
	err := run()
	if err != nil {
		os.Exit(1)
	}
}

func run() error {
	// TODO(@Wondertan): In practice we won't need all INFO loggers from IPFS/libp2p side
	//  so we would need to turn off them somewhere in `logs` package.
	logging.SetAllLoggers(logging.LevelInfo)
	return rootCmd.Execute()
}

var rootCmd = &cobra.Command{
	Use: "celestia [  full  ||  light  ] [subcommand]",
	Short: `
	  / ____/__  / /__  _____/ /_(_)___ _
	 / /   / _ \/ / _ \/ ___/ __/ / __  /
	/ /___/  __/ /  __(__  ) /_/ / /_/ /
	\____/\___/_/\___/____/\__/_/\__,_/
	`,
	Args: cobra.NoArgs,
}
