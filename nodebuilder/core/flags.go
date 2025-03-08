package core

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var (
	coreIPFlag         = "core.ip"
	corePortFlag       = "core.port"
	coreGRPCFlag       = "core.grpc.port"
	coreTLS            = "core.tls"
	coreXTokenPathFlag = "core.xtoken.path" //nolint:gosec
	coreEndpointsFlag  = "core.endpoints"
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
	flags.StringArray(
		coreEndpointsFlag,
		[]string{},
		"Specify core gRPC endpoints. Each endpoint can be specified in one of the following formats:\n"+
			"1. Simple format: 'ip:port' - uses defaults for TLS and X-Token\n"+
			"2. With TLS: 'ip:port:tls=true' or 'ip:port:tls=false'\n"+
			"3. Complete: 'ip:port:tls=true:xtoken=/path/to/token'\n"+
			"Example: --core.endpoints=127.0.0.1:9090 --core.endpoints=example.com:9091:tls=true:xtoken=/path/to/token",
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

	// Parse the primary endpoint
	coreIP := cmd.Flag(coreIPFlag).Value.String()
	if coreIP == "" {
		if cmd.Flag(corePortFlag).Changed {
			return fmt.Errorf("cannot specify gRPC port without specifying an IP address for --core.ip")
		}
	} else {
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
	}

	// Parse endpoints if specified
	if cmd.Flag(coreEndpointsFlag).Changed {
		endpoints, err := cmd.Flags().GetStringArray(coreEndpointsFlag)
		if err != nil {
			return fmt.Errorf("failed to get core.endpoints: %w", err)
		}

		// Parse and add each endpoint
		for _, endpointStr := range endpoints {
			// Use false as default TLS and empty string as default X-Token
			// Each endpoint should specify its own TLS and X-Token settings
			endpoint, err := parseEndpointString(endpointStr, false, "")
			if err != nil {
				return fmt.Errorf("failed to parse endpoint %s: %w", endpointStr, err)
			}

			cfg.Endpoints = append(cfg.Endpoints, endpoint)
		}
	}

	return cfg.Validate()
}

// parseEndpointString parses an endpoint string in one of the following formats:
// 1. "ip:port" - uses default TLS and X-Token settings
// 2. "ip:port:tls=true" or "ip:port:tls=false" - specifies TLS setting
// 3. "ip:port:tls=true:xtoken=/path/to/token" - specifies both TLS and X-Token path
// Also handles IPv6 addresses correctly: "[IPv6]:port:tls=true:xtoken=/path/to/token"
func parseEndpointString(endpointStr string, defaultTLS bool, defaultXToken string) (Endpoint, error) {
	// Initialize with defaults
	endpoint := Endpoint{
		TLSEnabled: defaultTLS,
		XTokenPath: defaultXToken,
	}

	// First, handle the special case of IPv6 addresses
	// Check if the string starts with '[' which indicates IPv6
	if strings.HasPrefix(endpointStr, "[") {
		// Find the closing bracket
		closeBracket := strings.Index(endpointStr, "]")
		if closeBracket == -1 {
			return Endpoint{}, fmt.Errorf("invalid IPv6 address format in %s, missing closing bracket", endpointStr)
		}

		// Extract the IPv6 address
		endpoint.IP = endpointStr[1:closeBracket]

		// The rest of the string should start with a colon followed by the port
		rest := endpointStr[closeBracket+1:]
		if !strings.HasPrefix(rest, ":") || len(rest) < 2 {
			return Endpoint{}, fmt.Errorf("invalid port format in IPv6 endpoint %s", endpointStr)
		}

		// Split the rest by colon
		restParts := strings.Split(rest[1:], ":")
		if len(restParts) < 1 {
			return Endpoint{}, fmt.Errorf("missing port in IPv6 endpoint %s", endpointStr)
		}

		endpoint.Port = restParts[0]

		// Process additional parts (TLS, X-Token)
		for i := 1; i < len(restParts); i++ {
			if strings.HasPrefix(restParts[i], "tls=") {
				tlsValue := strings.TrimPrefix(restParts[i], "tls=")
				switch tlsValue {
				case "true":
					endpoint.TLSEnabled = true
				case "false":
					endpoint.TLSEnabled = false
				default:
					return Endpoint{}, fmt.Errorf("invalid TLS value in endpoint %s, expected 'true' or 'false'", endpointStr)
				}
			} else if strings.HasPrefix(restParts[i], "xtoken=") {
				endpoint.XTokenPath = strings.TrimPrefix(restParts[i], "xtoken=")
			}
		}
	} else {
		// Regular IPv4 address or hostname
		parts := strings.Split(endpointStr, ":")
		if len(parts) < 2 {
			return Endpoint{}, fmt.Errorf("invalid endpoint format %s, expected at least 'ip:port'", endpointStr)
		}

		endpoint.IP = parts[0]
		endpoint.Port = parts[1]

		// Process additional parts (TLS, X-Token)
		for i := 2; i < len(parts); i++ {
			if strings.HasPrefix(parts[i], "tls=") {
				tlsValue := strings.TrimPrefix(parts[i], "tls=")
				switch tlsValue {
				case "true":
					endpoint.TLSEnabled = true
				case "false":
					endpoint.TLSEnabled = false
				default:
					return Endpoint{}, fmt.Errorf("invalid TLS value in endpoint %s, expected 'true' or 'false'", endpointStr)
				}
			} else if strings.HasPrefix(parts[i], "xtoken=") {
				endpoint.XTokenPath = strings.TrimPrefix(parts[i], "xtoken=")
			}
		}
	}

	return endpoint, nil
}
