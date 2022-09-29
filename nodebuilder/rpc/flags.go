package rpc

import (
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/api/gateway"
)

var (
	addrFlag = "gateway.addr"
	portFlag = "gateway.port"
)

// Flags gives a set of hardcoded node/rpc package flags.
func Flags() *flag.FlagSet {
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

// ParseFlags parses RPC flags from the given cmd and saves them to the passed config.
func ParseFlags(cmd *cobra.Command, cfg *gateway.Config) {
	addr := cmd.Flag(addrFlag).Value.String()
	if addr != "" {
		cfg.Address = addr
	}
	port := cmd.Flag(portFlag).Value.String()
	if port != "" {
		cfg.Port = port
	}
}
