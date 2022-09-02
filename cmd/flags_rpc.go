package cmd

import (
	"context"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/nodebuilder"
)

var (
	addrFlag = "rpc.addr"
	portFlag = "rpc.port"
)

// RPCFlags gives a set of hardcoded node/rpc package flags.
func RPCFlags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.String(
		addrFlag,
		"",
		"Set a custom RPC listen address (default: localhost)",
	)
	flags.String(
		portFlag,
		"",
		"Set a custom RPC port (default: 26658)",
	)

	return flags
}

// ParseRPCFlags parses RPC flags from the given cmd and applies values to Env.
func ParseRPCFlags(
	ctx context.Context,
	cmd *cobra.Command,
	cfg *nodebuilder.Config,
) (setCtx context.Context, err error) {
	defer func() {
		setCtx = WithNodeConfig(ctx, cfg)
	}()
	addr := cmd.Flag(addrFlag).Value.String()
	if addr != "" {
		cfg.RPC.Address = addr
	}
	port := cmd.Flag(portFlag).Value.String()
	if port != "" {
		cfg.RPC.Port = port
	}
	return
}
