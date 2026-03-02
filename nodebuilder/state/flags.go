package state

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var (
	keyringKeyNameFlag          = "keyring.keyname"
	keyringBackendFlag          = "keyring.backend"
	estimatorServiceAddressFlag = "estimator.service.address"
	estimatorServiceTLSFlag     = "estimator.service.tls"
	txWorkerAccountsFlag        = "tx.worker.accounts"
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
	flags.Int(
		txWorkerAccountsFlag,
		0,
		"used for queued submission. It defines how many accounts the TxClient uses for PayForBlob submissions.\n"+
			"0 disables queued submission. 1 uses the default account. Values greater than 1 automatically create "+
			"\"parallel-worker-*\" accounts and grant them fees using the default signer.",
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

	if cmd.Flag(estimatorServiceAddressFlag).Changed {
		cfg.EstimatorAddress = cmd.Flag(estimatorServiceAddressFlag).Value.String()
	}

	if cmd.Flag(estimatorServiceTLSFlag).Changed {
		cfg.EnableEstimatorTLS = true
	}

	if cmd.Flag(txWorkerAccountsFlag).Changed {
		value, err := strconv.Atoi(cmd.Flag(txWorkerAccountsFlag).Value.String())
		if err != nil {
			return err
		}

		cfg.TxWorkerAccounts = value
	}

	return nil
}
