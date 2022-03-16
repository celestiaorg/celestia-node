package main

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/cmd"
)

func init() {
	rootCmd.AddCommand(p2pCmd, headerCmd)
}

var rootCmd = &cobra.Command{
	Use: "cel-shed [subcommand]",
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true,
	},
}

func main() {
	err := run()
	if err != nil {
		os.Exit(1)
	}
}

func run() error {
	return rootCmd.ExecuteContext(cmd.WithEnv(context.Background()))
}
