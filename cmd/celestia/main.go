package main

import (
	"context"
	"os"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/config"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/app/encoding"
	"github.com/celestiaorg/celestia-node/cmd"
)

var encodingConfig encoding.EncodingConfig = encoding.MakeEncodingConfig(app.ModuleEncodingRegisters...)

var initClientCtx client.Context = client.Context{}.
	WithCodec(encodingConfig.Codec).
	WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
	WithTxConfig(encodingConfig.TxConfig).
	WithLegacyAmino(encodingConfig.Amino).
	WithInput(os.Stdin).
	WithAccountRetriever(types.AccountRetriever{}).
	WithBroadcastMode(flags.BroadcastBlock).
	WithHomeDir(app.DefaultNodeHome).
	WithViper("CELESTIA")

func init() {
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
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount(app.Bech32PrefixAccAddr, app.Bech32PrefixAccPub)
	cfg.Seal()

	ctx := context.WithValue(context.Background(), client.ClientContextKey, &initClientCtx)
	return rootCmd.ExecuteContext(cmd.WithEnv(ctx))
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
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		initClientCtx, err := client.ReadPersistentCommandFlags(initClientCtx, cmd.Flags())
		if err != nil {
			return err
		}
		initClientCtx, err = config.ReadFromClientConfig(initClientCtx)
		if err != nil {
			return err
		}

		if err = client.SetCmdClientContextHandler(initClientCtx, cmd); err != nil {
			return err
		}
		return nil
	},
}
