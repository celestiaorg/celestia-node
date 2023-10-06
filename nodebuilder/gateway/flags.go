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
		"Set a custom gateway port (default: 26659)",
	)

	return flags
}

// ParseFlags parses gateway flags from the given cmd and saves them to the passed config.
func ParseFlags(cmd *cobra.Command, cfg *Config) {
	enabled, err := cmd.Flags().GetBool(enabledFlag)
	if cmd.Flags().Changed(enabledFlag) && err == nil {
		cfg.Enabled = enabled
	}
	addr, port := cmd.Flag(addrFlag), cmd.Flag(portFlag)
	if !cfg.Enabled && (addr.Changed || port.Changed) {
		log.Warn("custom address or port provided without enabling gateway, setting config values")
	}
	addrVal := addr.Value.String()
	if addrVal != "" {
		cfg.Address = addrVal
	}
	portVal := port.Value.String()
	if portVal != "" {
		cfg.Port = portVal
	}
}
