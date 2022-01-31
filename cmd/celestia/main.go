package main

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/node"
)

func main() {
	err := run()
	if err != nil {
		os.Exit(1)
	}
}

func run() error {
	return rootCmd.ExecuteContext(cmd.WithEnv(context.Background()))
}

var rootCmd = NewRootCmd(nil)

func NewRootCmd(compAddr node.ComponentAdder) *cobra.Command {
	command := &cobra.Command{
		Use: "celestia [  bridge  ||  light  ] [subcommand]",
		Short: `
		  / ____/__  / /__  _____/ /_(_)___ _
		 / /   / _ \/ / _ \/ ___/ __/ / __  /
		/ /___/  __/ /  __(__  ) /_/ / /_/ /
		\____/\___/_/\___/____/\__/_/\__,_/
		`,
		Args: cobra.NoArgs,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}
	command.AddCommand(
		NewBridgeCommand(compAddr),
		NewLightCommand(compAddr),
		versionCmd,
	)
	command.SetHelpCommand(&cobra.Command{})
	return command
}
