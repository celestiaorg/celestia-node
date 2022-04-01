package main

import (
	"context"
	"os"

	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/spf13/cobra"
	"github.com/tendermint/spm/cosmoscmd"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-node/cmd"
)

func init() {
	// NOTE: this is absolutely necessary to ensure that the accounts are prefixed with `celes`
	cosmoscmd.SetPrefixes(app.AccountAddressPrefix)

	keyCmd := keys.Commands("")
	keyCmd.Short = "Manage your account's keys"

	rootCmd.AddCommand(
		bridgeCmd,
		lightCmd,
		fullCmd,
		keyCmd,
		versionCmd,
	)
	rootCmd.SetHelpCommand(&cobra.Command{})
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

var rootCmd = &cobra.Command{
	Use: "celestia [  bridge  ||  full ||  light  ] [subcommand]",
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
