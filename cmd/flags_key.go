package cmd

import (
	"context"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/nodebuilder"
)

var keyringAccNameFlag = "keyring.accname"

func KeyFlags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.String(keyringAccNameFlag, "", "Directs node's keyring signer to use the key prefixed with the "+
		"given string.")
	return flags
}

func ParseKeyFlags(ctx context.Context, cmd *cobra.Command, cfg *nodebuilder.Config) (setCtx context.Context) {
	defer func() {
		setCtx = WithNodeConfig(ctx, cfg)
	}()
	keyringAccName := cmd.Flag(keyringAccNameFlag).Value.String()
	if keyringAccName != "" {
		cfg.State.KeyringAccName = keyringAccName
	}
	return
}
