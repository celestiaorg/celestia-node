package core

import (
	"fmt"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var (
	ipFlag       = "core.ip"
	rpcIPFlag    = "core.rpc.ip"
	grpcIPFlag   = "core.grpc.ip"
	rpcPortFlag  = "core.rpc.port"
	grpcPortFlag = "core.grpc.port"
)

// Flags gives a set of hardcoded Core flags.
func Flags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.String(
		ipFlag,
		"",
		"Indicates node to connect to the given core node's RPC and gRPC. "+
			"NOTE: If this flag is set, the core.rpc.ip and core.grpc.ip flags cannot be set. "+
			"Example: <ip>, 127.0.0.1. <dns>, subdomain.domain.tld "+
			"Assumes RPC port 26657 and gRPC port 9090 as default unless otherwise specified.",
	)
	flags.String(
		rpcIPFlag,
		"",
		"Indicates node to connect to the given core node's RPC. "+
			"NOTE: If this flag is set, the core.ip flag cannot be set. "+
			"Example: <ip>, 127.0.0.1. <dns>, subdomain.domain.tld "+
			"Assumes RPC port 26657 and gRPC port 9090 as default unless otherwise specified.",
	)
	flags.String(
		grpcIPFlag,
		"",
		"Indicates node to connect to the given core node's gRPC. "+
			"NOTE: If this flag is set, the core.ip flag cannot be set. "+
			"Example: <ip>, 127.0.0.1. <dns>, subdomain.domain.tld "+
			"Assumes RPC port 26657 and gRPC port 9090 as default unless otherwise specified.",
	)
	flags.String(
		rpcPortFlag,
		"26657",
		"Set a custom RPC port for the core node connection. The --core.rpc.ip or --core.ip flag must also be provided.",
	)
	flags.String(
		grpcPortFlag,
		"9090",
		"Set a custom gRPC port for the core node connection. The --core.grpc.ip or --core.ip flag must also be provided.",
	)
	return flags
}

// ParseFlags parses Core flags from the given cmd and saves them to the passed config.
func ParseFlags(cmd *cobra.Command, cfg *Config) error {
	coreIP := cmd.Flag(ipFlag).Value.String()
	coreRPCIP, coreGRPCIP := cmd.Flag(rpcIPFlag).Value.String(), cmd.Flag(grpcIPFlag).Value.String()

	// Check if core.ip is specified along with core.rpc.ip or core.grpc.ip
	if coreIP != "" && (cmd.Flag(rpcIPFlag).Changed || cmd.Flag(grpcIPFlag).Changed) {
		return fmt.Errorf("cannot specify core.ip and core.rpc.ip or core.grpc.ip flags together")
	}

	// Validate IP addresses and port settings
	if coreIP == "" { // No core.ip specified
		if coreRPCIP == "" {
			if cmd.Flag(rpcPortFlag).Changed {
				return fmt.Errorf("cannot specify RPC ports without specifying an IP address for --core.rpc.ip")
			}
			if cmd.Flag(grpcIPFlag).Changed {
				return fmt.Errorf("setting gRPC IP requires also specifying an RPC IP. If they are identical use --core.ip instead")
			}
		}
		if coreGRPCIP == "" {
			if cmd.Flag(grpcPortFlag).Changed {
				return fmt.Errorf("cannot specify gRPC ports without specifying an IP address for --core.grpc.ip")
			}
			if cmd.Flag(rpcIPFlag).Changed {
				return fmt.Errorf("setting RPC IP requires also specifying a gRPC IP. If they are identical use --core.ip instead")
			}
		}
	}

	// Assign IP addresses
	if coreIP != "" {
		cfg.RPCIP, cfg.GRPCIP = coreIP, coreIP
	} else {
		cfg.RPCIP, cfg.GRPCIP = coreRPCIP, coreGRPCIP
	}

	// Assign ports
	cfg.RPCPort, cfg.GRPCPort = cmd.Flag(rpcPortFlag).Value.String(), cmd.Flag(grpcPortFlag).Value.String()

	return cfg.Validate()
}
