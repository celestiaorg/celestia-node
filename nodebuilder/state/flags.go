package state

import (
	"fmt"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var (
	keyringAccNameFlag = "keyring.accname"
	keyringBackendFlag = "keyring.backend"

	granterEnabledFlag = "granter.enabled"
)

// Flags gives a set of hardcoded State flags.
func Flags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.String(keyringAccNameFlag, "", "Directs node's keyring signer to use the key prefixed with the "+
		"given string.")
	flags.String(keyringBackendFlag, defaultKeyringBackend, fmt.Sprintf("Directs node's keyring signer to use the given "+
		"backend. Default is %s.", defaultKeyringBackend))

	flags.Bool(granterEnabledFlag, false, fmt.Sprintf("Allows to run the node in a grantee mode. Default is %v.", false))
	return flags
}

// ParseFlags parses State flags from the given cmd and saves them to the passed config.
func ParseFlags(cmd *cobra.Command, cfg *Config) {
	keyringAccName := cmd.Flag(keyringAccNameFlag).Value.String()
	if keyringAccName != "" {
		cfg.KeyringAccName = keyringAccName
	}

	cfg.KeyringBackend = cmd.Flag(keyringBackendFlag).Value.String()

	enabled, err := cmd.Flags().GetBool(granterEnabledFlag)
	if err != nil {
		panic(err)
	}
	cfg.GranterEnabled = enabled
}
