package state

import (
	"fmt"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var (
	keyringAccNameFlag = "keyring.accname"
	keyringBackendFlag = "keyring.backend"

	granterAddressFlag = "granter.address"
)

// Flags gives a set of hardcoded State flags.
func Flags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.String(keyringAccNameFlag, "", "Directs node's keyring signer to use the key prefixed with the "+
		"given string.")
	flags.String(keyringBackendFlag, defaultKeyringBackend, fmt.Sprintf("Directs node's keyring signer to use the given "+
		"backend. Default is %s.", defaultKeyringBackend))

	flags.String(granterAddressFlag, "", "Account address that will pay for all transactions submitted from the node.")
	return flags
}

// ParseFlags parses State flags from the given cmd and saves them to the passed config.
func ParseFlags(cmd *cobra.Command, cfg *Config) error {
	keyringAccName := cmd.Flag(keyringAccNameFlag).Value.String()
	if keyringAccName != "" {
		cfg.KeyringAccName = keyringAccName
	}

	cfg.KeyringBackend = cmd.Flag(keyringBackendFlag).Value.String()

	addr := cmd.Flag(granterAddressFlag).Value.String()
	if addr == "" {
		return nil
	}

	sdkAddress, err := sdktypes.AccAddressFromBech32(addr)
	cfg.GranterAddress = sdkAddress
	return err
}
