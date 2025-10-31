package core

import (
	"fmt"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var (
	coreIPFlag         = "core.ip"
	corePortFlag       = "core.port"
	coreTLS            = "core.tls"
	coreXTokenPathFlag = "core.xtoken.path" //nolint:gosec
)

// Flags gives a set of hardcoded Core flags.
func Flags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.String(
		coreIPFlag,
		"",
		"Indicates node to connect to the given core node. "+
			"Example: <ip>, 127.0.0.1. <dns>, subdomain.domain.tld "+
			"Assumes gRPC port 9090 as default unless otherwise specified.",
	)
	flags.String(
		corePortFlag,
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
		"specifies the directory path containing the JSON file with the X-Token for gRPC authentication. "+
			"The JSON file can be named either 'xtoken.json' or 'x-token.json'. "+
			"The JSON file should have a key-value pair where the key is 'x-token' (preferred) or 'xtoken', "+
			"and the value is the authentication token. "+
			"NOTE: the path is parsed only if core.tls enabled. "+
			"If left empty, the client will not include the X-Token in its requests.",
	)
	return flags
}

// ParseFlags parses Core flags from the given cmd and saves them to the passed config.
func ParseFlags(
	cmd *cobra.Command,
	cfg *Config,
) error {
	coreIP := cmd.Flag(coreIPFlag).Value.String()
	if coreIP == "" {
		if cmd.Flag(corePortFlag).Changed {
			return fmt.Errorf("cannot specify gRPC port without specifying an IP address for --core.ip")
		}
		return nil
	}

	// Set port from flag if explicitly changed, otherwise preserve config value or use default
	if cmd.Flag(corePortFlag).Changed {
		grpc := cmd.Flag(corePortFlag).Value.String()
		cfg.Port = grpc
	} else if cfg.Port == "" {
		// If no config value and flag not changed, use default
		cfg.Port = DefaultPort
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
