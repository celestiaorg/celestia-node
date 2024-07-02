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

	flags.String(keyringKeyNameFlag, DefaultAccountName,
		fmt.Sprintf("Directs node's keyring signer to use the key prefixed with the "+
			"given string. Default is %s", DefaultAccountName))
	flags.String(keyringBackendFlag, defaultKeyringBackend,
		fmt.Sprintf("Directs node's keyring signer to use the given "+
			"backend. Default is %s.", defaultKeyringBackend))
	return flags
}

// ParseFlags parses State flags from the given cmd and saves them to the passed config.
func ParseFlags(cmd *cobra.Command, cfg *Config) {
	cfg.KeyringKeyName = cmd.Flag(keyringKeyNameFlag).Value.String()
	cfg.KeyringBackend = cmd.Flag(keyringBackendFlag).Value.String()
}
