package core

import (
	"fmt"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var (
	coreIPFlag               = "core.ip"
	corePortFlag             = "core.port"
	coreGRPCFlag             = "core.grpc.port"
	coreTLS                  = "core.tls"
	coreXTokenPathFlag       = "core.xtoken.path" //nolint:gosec
	coreEstimatorAddressFlag = "core.estimator.address"
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
	flags.String(
		coreGRPCFlag,
		"",
		"Set a custom gRPC port for the core node connection.WARNING: --core.grpc.port is deprecated. "+
			"Please use --core.port instead",
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
	flags.String(
		coreEstimatorAddressFlag,
		"",
		"specifies the endpoint of the third-party service that should be used to calculate"+
			"the gas price and gas. Format: <address>:<port>. Default connection to the consensus node will be used if "+
			"left empty.",
	)
	return flags
}

// ParseFlags parses Core flags from the given cmd and saves them to the passed config.
func ParseFlags(
	cmd *cobra.Command,
	cfg *Config,
) error {
	if cmd.Flag(coreGRPCFlag).Changed {
		return fmt.Errorf("the flag is deprecated. Please use --core.port instead")
	}

	coreIP := cmd.Flag(coreIPFlag).Value.String()
	if coreIP == "" {
		if cmd.Flag(corePortFlag).Changed {
			return fmt.Errorf("cannot specify gRPC port without specifying an IP address for --core.ip")
		}
		return nil
	}

	if cmd.Flag(corePortFlag).Changed {
		grpc := cmd.Flag(corePortFlag).Value.String()
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

	if cmd.Flag(coreEstimatorAddressFlag).Changed {
		addr := cmd.Flag(coreEstimatorAddressFlag).Value.String()
		cfg.FeeEstimatorAddress = EstimatorAddress(addr)
	}
	return cfg.Validate()
}
