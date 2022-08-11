package cmd

import (
	"context"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/node"
)

var keyringAccNameFlag = "keyring.accname"

func KeyFlags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.String(keyringAccNameFlag, "", "Directs node's keyring signer to use the key prefixed with the "+
		"given string.")
	return flags
}

func ParseKeyFlags(ctx context.Context, cmd *cobra.Command) context.Context {
	keyringAccName := cmd.Flag(keyringAccNameFlag).Value.String()
	if keyringAccName != "" {
		return AddOptions(ctx, node.WithKeyringAccName(keyringAccName))
	}
	return ctx
}
