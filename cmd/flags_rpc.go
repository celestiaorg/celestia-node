package cmd

import (
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/node"
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
		"localhost",
		"Set a custom RPC listen address (default: localhost)",
	)
	flags.String(
		portFlag,
		"26658",
		"Set a custom RPC port (default: 26658)",
	)

	return flags
}

// ParseRPCFlags parses RPC flags from the given cmd and applies values to Env.
func ParseRPCFlags(cmd *cobra.Command, env *Env) error {
	addr := cmd.Flag(addrFlag).Value.String()
	if addr != "" {
		env.AddOptions(node.WithRPCAddress(addr))
	}
	port := cmd.Flag(portFlag).Value.String()
	if port != "" {
		env.AddOptions(node.WithRPCPort(port))
	}
	return nil
}
