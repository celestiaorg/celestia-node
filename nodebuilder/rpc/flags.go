package rpc

import (
	"fmt"

	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var (
	log = logging.Logger("rpc")

	addrFlag               = "rpc.addr"
	portFlag               = "rpc.port"
	authFlag               = "rpc.skip-auth"
	corsEnabledFlag        = "rpc.cors-enabled"
	corsAllowedOriginsFlag = "rpc.cors-allowed-origins"
	corsAllowedMethodsFlag = "rpc.cors-allowed-methods"
	corsAllowedHeadersFlag = "rpc.cors-allowed-headers"
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
		corsEnabledFlag,
		false,
		"Enable CORS for RPC server",
	)
	flags.StringSlice(
		corsAllowedOriginsFlag,
		[]string{},
		"Comma-separated list of origins allowed to access the RPC server via CORS (cors enabled default: empty)",
	)
	flags.StringSlice(
		corsAllowedMethodsFlag,
		[]string{},
		fmt.Sprintf("Comma-separated list of HTTP methods allowed for CORS (cors enabled default: %s)", defaultAllowedMethods),
	)
	flags.StringSlice(
		corsAllowedHeadersFlag,
		[]string{},
		fmt.Sprintf("Comma-separated list of HTTP headers allowed for CORS (cors enabled default: %s)", defaultAllowedHeaders),
	)

	return flags
}

// ParseFlags parses RPC flags from the given cmd and saves them to the passed config.
func ParseFlags(cmd *cobra.Command, cfg *Config) error {
	corsSettingsProvided := false
	if addr, err := cmd.Flags().GetString(addrFlag); err != nil {
		return err
	} else if addr != "" {
		cfg.Address = addr
	}
	if port, err := cmd.Flags().GetString(portFlag); err != nil {
		return err
	} else if port != "" {
		cfg.Port = port
	}
	if val, err := cmd.Flags().GetBool(authFlag); err != nil {
		return err
	} else if val {
		log.Warn("RPC authentication disabled (--rpc.skip-auth)")
		cfg.SkipAuth = true
	}

	// CORS setting
	if val, err := cmd.Flags().GetBool(corsEnabledFlag); err != nil {
		return err
	} else if val {
		log.Info("CORS enabled (--rpc.cors-enabled)")
		cfg.CORS.Enabled = true
	}

	if origins, err := cmd.Flags().GetStringSlice(corsAllowedOriginsFlag); err != nil {
		return err
	} else if len(origins) > 0 {
		if cfg.CORS.Enabled {
			cfg.CORS.AllowedOrigins = origins
		}
		corsSettingsProvided = true
	}

	if methods, err := cmd.Flags().GetStringSlice(corsAllowedMethodsFlag); err != nil {
		return err
	} else if len(methods) > 0 {
		cfg.CORS.AllowedMethods = methods
		corsSettingsProvided = true
	} else if cfg.CORS.Enabled {
		log.Info("CORS enabled but no methods provided, using default")
		cfg.CORS.AllowedMethods = defaultAllowedMethods
	}

	if headers, err := cmd.Flags().GetStringSlice(corsAllowedHeadersFlag); err != nil {
		return err
	} else if len(headers) > 0 {
		cfg.CORS.AllowedHeaders = headers
		corsSettingsProvided = true
	} else if cfg.CORS.Enabled {
		log.Info("CORS enabled but no headers provided, using default")
		cfg.CORS.AllowedHeaders = defaultAllowedHeaders
	}

	if corsSettingsProvided && !cfg.CORS.Enabled {
		return fmt.Errorf("CORS settings provided but CORS is not enabled. Set --rpc.cors-enabled=true to apply these settings.")
	}
	if cfg.CORS.Enabled && !cfg.SkipAuth && len(cfg.CORS.AllowedOrigins) == 0 {
		log.Warn("CORS enabled without allowed origins. Set --rpc.cors-allowed-origins to enable cross-origin requests.")
	}
	if !cfg.SkipAuth && corsSettingsProvided {
		log.Warn("CORS settings provided but authentication is enabled. CORS settings may not work as expected.")
	}
	return nil
}
