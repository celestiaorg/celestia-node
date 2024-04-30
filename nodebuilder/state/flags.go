package state

import (
	"fmt"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var (
	keyringKeyNameFlag = "keyring.keyname"
	keyringBackendFlag = "keyring.backend"
)

// Flags gives a set of hardcoded State flags.
func Flags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.StringSlice(keyringKeyNameFlag, []string{}, "Directs node's keyring signer to use the key by the "+
		"given name. The first key in the list will be the default key that will be used to sign the transactions. "+
		"All other keys can be used to sign transactions, given that the user specifies it in the transaction options.")
	flags.String(keyringBackendFlag, defaultKeyringBackend, fmt.Sprintf("Directs node's keyring signer to use the given "+
		"backend. Default is %s.", defaultKeyringBackend))
	return flags
}

// ParseFlags parses State flags from the given cmd and saves them to the passed config.
func ParseFlags(cmd *cobra.Command, cfg *Config) error {
	keyringKeyNames, err := cmd.Flags().GetStringSlice(keyringKeyNameFlag)
	if err != nil {
		return err
	}
	cfg.KeyringKeyNames = keyringKeyNames

	cfg.KeyringBackend = cmd.Flag(keyringBackendFlag).Value.String()
	return err
}
