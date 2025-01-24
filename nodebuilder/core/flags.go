package core

import (
	"fmt"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var (
	coreFlag           = "core.ip"
	coreGRPCFlag       = "core.grpc.port"
	coreTLS            = "core.tls"
	coreXTokenPathFlag = "core.xtoken.path" //nolint:gosec
)

// Flags gives a set of hardcoded Core flags.
func Flags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.String(
		coreFlag,
		"",
		"Indicates node to connect to the given core node. "+
			"Example: <ip>, 127.0.0.1. <dns>, subdomain.domain.tld "+
			"Assumes gRPC port 9090 as default unless otherwise specified.",
	)
	flags.String(
		coreGRPCFlag,
		DefaultPort,
		"Set a custom gRPC port for the core node connection. The --core.ip flag must also be provided.",
	)
	flags.Bool(
		coreTLS,
		false,
		"Specifies whether TLS is enabled or not. Default: false",
	)
	flags.String(
		coreXTokenPathFlag,
		"",
		"specifies the file path to the JSON file containing the X-Token for gRPC authentication. "+
			"The JSON file should have a key-value pair where the key is 'x-token' and the value is the authentication token. "+
			"NOTE: the path is parsed only if coreTLS enabled."+
			"If left empty, the client will not include the X-Token in its requests.",
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
		if cmd.Flag(coreGRPCFlag).Changed {
			return fmt.Errorf("cannot specify gRPC port without specifying an IP address for --core.ip")
		}
		return nil
	}

	if cmd.Flag(coreGRPCFlag).Changed {
		grpc := cmd.Flag(coreGRPCFlag).Value.String()
		cfg.Port = grpc
	}

	enabled, err := cmd.Flags().GetBool(coreTLS)
	if err != nil {
		return err
	}

	if enabled {
		cfg.TLSEnabled = true
		if cmd.Flag(coreXTokenPathFlag).Changed {
			path := cmd.Flag(coreXTokenPathFlag).Value.String()
			cfg.XTokenPath = path
		}
	}
	cfg.IP = coreIP
	return cfg.Validate()
}
