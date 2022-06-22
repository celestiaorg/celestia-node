package main

import (
	"context"
	crypto_rand "crypto/rand"
	"encoding/binary"
	math_rand "math/rand"
	"os"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-node/cmd"
)

func init() {
	// This is necessary to ensure that the account addresses are correctly prefixed
	// as in the celestia application.
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount(app.Bech32PrefixAccAddr, app.Bech32PrefixAccPub)
	cfg.Seal()

	rootCmd.AddCommand(
		bridgeCmd,
		lightCmd,
		fullCmd,
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
	seed := make([]byte, 32)
	_, err := crypto_rand.Read(seed)
	if err != nil {
		return err
	}
	math_rand.Seed(int64(binary.LittleEndian.Uint64(seed)))

	return rootCmd.ExecuteContext(cmd.WithEnv(context.Background()))
}

var rootCmd = &cobra.Command{
	Use: "celestia [  bridge  ||  full ||  light  ] [subcommand]",
	Short: `
		____      __          __  _
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
