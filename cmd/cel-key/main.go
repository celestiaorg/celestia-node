package main

import (
	"context"
	"os"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/config"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-app/v2/app"
	"github.com/celestiaorg/celestia-app/v2/app/encoding"
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

		if !cmd.Flag(flags.FlagKeyringBackend).Changed {
			err = cmd.Flag(flags.FlagKeyringBackend).Value.Set(keyring.BackendTest)
			if err != nil {
				return err
			}
			cmd.Flag(flags.FlagKeyringBackend).Changed = true
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
	ctx := context.WithValue(context.Background(), client.ClientContextKey, &initClientCtx)
	return rootCmd.ExecuteContext(ctx)
}
