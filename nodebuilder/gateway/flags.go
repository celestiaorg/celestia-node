package gateway

import (
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var (
	enabledFlag = "gateway"
	addrFlag    = "gateway.addr"
	portFlag    = "gateway.port"
)

// Flags gives a set of hardcoded node/gateway package flags.
func Flags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.Bool(
		enabledFlag,
		false,
		"Enables the REST gateway",
	)
	flags.String(
		addrFlag,
		"",
		"Set a custom gateway listen address (default: localhost)",
	)
	flags.String(
		portFlag,
		"",
		"Set a custom gateway port (default: 26658)",
	)

	return flags
}

// ParseFlags parses gateway flags from the given cmd and saves them to the passed config.
func ParseFlags(cmd *cobra.Command, cfg *Config) {
	addr := cmd.Flag(addrFlag).Value.String()
	if addr != "" {
		cfg.Address = addr
	}
	port := cmd.Flag(portFlag).Value.String()
	if port != "" {
		cfg.Port = port
	}
	enabled, err := cmd.Flags().GetBool(enabledFlag)
	if err == nil {
		cfg.Enabled = enabled
	}
}
