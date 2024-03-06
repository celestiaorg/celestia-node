package core

import (
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var (
	coreIpFlag = "core.ip" // TODO: @ramin - we should deprecate this

	rpcHostFlag = "core.rpc.host"
	rpcPortFlag = "core.rpc.port"

	grpcHostFlag = "core.grpc.host"
	grpcPortFlag = "core.grpc.port"
	grpcCertFlag = "core.grpc.cert"
)

// Flags gives a set of hardcoded Core flags.
//
//nolint:goconst
func Flags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.String(
		coreIpFlag,
		"",
		"Indicates node to connect to the given core node's RPC and gRPC. "+
			"NOTE: If this flag is set, the core.rpc.ip and core.grpc.ip flags cannot be set. "+
			"Example: <ip>, 127.0.0.1. <dns>, subdomain.domain.tld "+
			"Assumes RPC port 26657 and gRPC port 9090 as default unless otherwise specified.",
	)
	// RPC configuration flags
	flags.String(
		rpcHostFlag,
		"",
		"Indicates node to connect to the given core node's RPC. "+
			"NOTE: If this flag is set, the core.ip flag cannot be set. "+
			"Example: <ip>, 127.0.0.1. <dns>, subdomain.domain.tld "+
			"Assumes RPC port 26657 as default unless otherwise specified.",
	)
	flags.String(
		rpcPortFlag,
		DefaultRPCPort,
		"Set a custom RPC port for the core node connection.",
	)
	// gRPC configuration flags
	flags.String(
		grpcHostFlag,
		"",
		"Indicates node to connect to the given core node's gRPC. "+
			"NOTE: If this flag is set, the core.ip flag cannot be set. "+
			"Example: <ip>, 127.0.0.1. <dns>, subdomain.domain.tld "+
			"Assumes RPC gRPC port 9090 as default unless otherwise specified.",
	)

	flags.String(
		grpcPortFlag,
		DefaultGRPCPort,
		"Set a custom gRPC port for the core node connection.",
	)
	flags.String(
		grpcCertFlag,
		"",
		"Set a path to a gRPC certificate for the core node connection.",
	)
	return flags
}

// ParseFlags parses Core flags from the given cmd and saves them to the passed config.
func ParseFlags(cmd *cobra.Command, cfg *Config) error {
	cfg.IP = cmd.Flag(coreIpFlag).Value.String()
	cfg.RPC.Host = cmd.Flag(rpcHostFlag).Value.String()
	cfg.RPC.Port = cmd.Flag(rpcPortFlag).Value.String()

	cfg.GRPC.Host = cmd.Flag(grpcHostFlag).Value.String()
	cfg.GRPC.Port = cmd.Flag(grpcPortFlag).Value.String()
	cfg.GRPC.Cert = cmd.Flag(grpcCertFlag).Value.String()

	return cfg.Validate()
}
