package core

import (
	"fmt"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var (
	coreFlag     = "core.ip"
	coreRPCFlag  = "core.rpc.port"
	coreGRPCFlag = "core.grpc.port"
)

// Flags gives a set of hardcoded Core flags.
func Flags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.String(
		coreFlag,
		"",
		"Indicates node to connect to the given core node. "+
			"Example: <ip>, 127.0.0.1. Assumes RPC port 26657 and gRPC port 9090 as default unless otherwise specified.",
	)
	flags.String(
		coreRPCFlag,
		"26657",
		"Set a custom RPC port for the core node connection. The --core.ip flag must also be provided.",
	)
	flags.String(
		coreGRPCFlag,
		"9090",
		"Set a custom gRPC port for the core node connection. The --core.ip flag must also be provided.",
	)
	return flags
}

// ParseFlags parses Core flags from the given cmd and saves them to the passed config.
func ParseFlags(
	cmd *cobra.Command,
	cfg *Config,
) error {
	coreIP := cmd.Flag(coreFlag).Value.String()
	if coreIP == "" {
		if cmd.Flag(coreGRPCFlag).Changed || cmd.Flag(coreRPCFlag).Changed {
			return fmt.Errorf("cannot specify RPC/gRPC ports without specifying an IP address for --core.ip")
		}
		return nil
	}

	rpc := cmd.Flag(coreRPCFlag).Value.String()
	grpc := cmd.Flag(coreGRPCFlag).Value.String()

	cfg.IP = coreIP
	cfg.RPCPort = rpc
	cfg.GRPCPort = grpc
	return nil
}
