package rpc

import (
	"fmt"

	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var (
	log              = logging.Logger("rpc")
	addrFlag         = "rpc.addr"
	portFlag         = "rpc.port"
	authFlag         = "rpc.skip-auth"
	tlsEnabledFlag   = "rpc.tls"
	tlsCertPathFlag  = "rpc.tls-cert"
	tlsKeyPathFlag   = "rpc.tls-key"
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
	flags.Bool(
		authFlag,
		false,
		"Skips authentication for RPC requests",
	)
	flags.Bool(
		tlsEnabledFlag,
		false,
		"Enable TLS for RPC server",
	)
	flags.String(
		tlsCertPathFlag,
		"",
		"Path to TLS certificate file",
	)
	flags.String(
		tlsKeyPathFlag,
		"",
		"Path to TLS private key file",
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
	ok, err := cmd.Flags().GetBool(authFlag)
	if err != nil {
		panic(err)
	}
	if ok {
		log.Warn("RPC authentication is disabled")
		cfg.SkipAuth = true
	}

	// TLS flags
	tlsEnabled, err := cmd.Flags().GetBool(tlsEnabledFlag)
	if err != nil {
		panic(err)
	}
	cfg.TLSEnabled = tlsEnabled
	
	certPath := cmd.Flag(tlsCertPathFlag).Value.String()
	if certPath != "" {
		cfg.TLSCertPath = certPath
	}
	
	keyPath := cmd.Flag(tlsKeyPathFlag).Value.String()
	if keyPath != "" {
		cfg.TLSKeyPath = keyPath
	}
}
