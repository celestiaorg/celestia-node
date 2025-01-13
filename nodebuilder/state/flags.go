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

	flags.String(keyringKeyNameFlag, DefaultKeyName,
		fmt.Sprintf("Directs node's keyring signer to use the key prefixed with the "+
			"given string. Default is %s", DefaultKeyName))
	flags.String(keyringBackendFlag, defaultBackendName,
		fmt.Sprintf("Directs node's keyring signer to use the given "+
			"backend. Default is %s.", defaultBackendName))
	return flags
}

// ParseFlags parses State flags from the given cmd and saves them to the passed config.
func ParseFlags(cmd *cobra.Command, cfg *Config) {
	if cmd.Flag(keyringKeyNameFlag).Changed {
		cfg.DefaultKeyName = cmd.Flag(keyringKeyNameFlag).Value.String()
	}
	if cmd.Flag(keyringBackendFlag).Changed {
		cfg.DefaultBackendName = cmd.Flag(keyringBackendFlag).Value.String()
	}
}
