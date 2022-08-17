package main

import (
	"context"
	"os"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/config"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/keys"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/app/encoding"
)

var encodingConfig = encoding.MakeConfig(app.ModuleEncodingRegisters...)

var initClientCtx = client.Context{}.
	WithCodec(encodingConfig.Codec).
	WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
	WithTxConfig(encodingConfig.TxConfig).
	WithLegacyAmino(encodingConfig.Amino).
	WithInput(os.Stdin).
	WithAccountRetriever(types.AccountRetriever{}).
	WithBroadcastMode(flags.BroadcastBlock).
	WithHomeDir(app.DefaultNodeHome).
	WithViper("CELESTIA")

var rootCmd = keys.Commands("~")

func init() {
	rootCmd.PersistentFlags().AddFlagSet(DirectoryFlags())
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		initClientCtx, err := client.ReadPersistentCommandFlags(initClientCtx, cmd.Flags())
		if err != nil {
			return err
		}
		initClientCtx, err = config.ReadFromClientConfig(initClientCtx)
		if err != nil {
			return err
		}

		if err := client.SetCmdClientContextHandler(initClientCtx, cmd); err != nil {
			return err
		}

		return ParseDirectoryFlags(cmd)
	}
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
	cfg.SetBech32PrefixForValidator(app.Bech32PrefixValAddr, app.Bech32PrefixValPub)
	cfg.Seal()

	ctx := context.WithValue(context.Background(), client.ClientContextKey, &initClientCtx)
	return rootCmd.ExecuteContext(ctx)
}
