package rpc

import (
	"fmt"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var (
	addrFlag = "rpc.addr"
	portFlag = "rpc.port"
)

// Flags gives a set of hardcoded node/rpc package flags.
func Flags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.String(
		addrFlag,
		"",
		fmt.Sprintf("Set a custom RPC listen address (default: %s)", defaultBindAddress),
	)
	flags.String(
		portFlag,
		"",
		fmt.Sprintf("Set a custom RPC port (default: %s)", defaultPort),
	)

	return flags
}

// ParseFlags parses RPC flags from the given cmd and saves them to the passed config.
func ParseFlags(cmd *cobra.Command, cfg *Config) {
	addr := cmd.Flag(addrFlag).Value.String()
	if addr != "" {
		cfg.Address = addr
	}
	port := cmd.Flag(portFlag).Value.String()
	if port != "" {
		cfg.Port = port
	}
}
