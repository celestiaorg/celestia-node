package state

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var (
	keyringKeyNameFlag          = "keyring.keyname"
	keyringBackendFlag          = "keyring.backend"
	keyringSeedsFlag            = "keyring.seeds"
	keyringSeedsFileFlag        = "keyring.seeds.file"
	estimatorServiceAddressFlag = "estimator.service.address"
	estimatorServiceTLSFlag     = "estimator.service.tls"
)

// Flags gives a set of hardcoded State flags.
func Flags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.String(keyringKeyNameFlag, DefaultKeyName,
		fmt.Sprintf("Directs node's keyring signer to use the key prefixed with the "+
			"given string. Default is %s", DefaultKeyName))
	flags.String(keyringBackendFlag, defaultBackendName,
		fmt.Sprintf("Directs node's keyring signer to use the given "+
			"backend. Default is %s.", defaultBackendName))
	flags.String(keyringSeedsFlag, "",
		"Specifies the seeds for the keyring signer. It generates new seeds if not provided.")
	flags.String(keyringSeedsFileFlag, "",
		"Specifies the file path for the seeds for the keyring signer. It generates new seeds if not provided.")
	flags.String(
		estimatorServiceAddressFlag,
		"",
		"specifies the endpoint of the third-party service that should be used to calculate"+
			"the gas price and gas. Format: <address>:<port>. Default connection to the consensus node will be "+
			"used if left empty.",
	)
	flags.Bool(
		estimatorServiceTLSFlag,
		false,
		"enables TLS for the estimator service gRPC connection",
	)

	return flags
}

// ParseFlags parses State flags from the given cmd and saves them to the passed config.
func ParseFlags(cmd *cobra.Command, cfg *Config) error {
	if cmd.Flag(keyringKeyNameFlag).Changed {
		cfg.DefaultKeyName = cmd.Flag(keyringKeyNameFlag).Value.String()
	}
	if cmd.Flag(keyringBackendFlag).Changed {
		cfg.DefaultBackendName = cmd.Flag(keyringBackendFlag).Value.String()
	}

	if cmd.Flag(keyringSeedsFlag).Changed {
		cfg.Seeds = cmd.Flag(keyringSeedsFlag).Value.String()
	}

	if cmd.Flag(keyringSeedsFileFlag).Changed {
		seedsBytes, err := os.ReadFile(cmd.Flag(keyringSeedsFileFlag).Value.String())
		if err != nil {
			return fmt.Errorf("failed to read seeds file: %w", err)
		}
		cfg.Seeds = string(seedsBytes)
	}

	if cmd.Flag(estimatorServiceAddressFlag).Changed {
		cfg.EstimatorAddress = cmd.Flag(estimatorServiceAddressFlag).Value.String()
	}

	if cmd.Flag(estimatorServiceTLSFlag).Changed {
		cfg.EnableEstimatorTLS = true
	}

	return nil
}
