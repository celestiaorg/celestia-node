package header

import (
	"encoding/hex"
	"fmt"

	"github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var (
	headersTrustedHashFlag  = "headers.trusted-hash"
	headersTrustedPeersFlag = "headers.trusted-peers"
)

// Flags gives a set of hardcoded Header package flags.
func Flags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.AddFlagSet(TrustedPeersFlags())
	flags.AddFlagSet(TrustedHashFlags())

	return flags
}

// ParseFlags parses Header package flags from the given cmd and applies them to the passed config.
func ParseFlags(cmd *cobra.Command, cfg *Config) error {
	if err := ParseTrustedHashFlags(cmd, cfg); err != nil {
		return err
	}
	return ParseTrustedPeerFlags(cmd, cfg)
}

// TrustedPeersFlags returns a set of flags.
func TrustedPeersFlags() *flag.FlagSet {
	flags := &flag.FlagSet{}
	flags.StringSlice(
		headersTrustedPeersFlag,
		nil,
		"Multiaddresses of a reliable peers to fetch headers from. (Format: multiformats.io/multiaddr)",
	)
	return flags
}

// ParseTrustedPeerFlags parses Header package flags from the given cmd and applies them to the
// passed config.
func ParseTrustedPeerFlags(
	cmd *cobra.Command,
	cfg *Config,
) error {
	tpeers, err := cmd.Flags().GetStringSlice(headersTrustedPeersFlag)
	if err != nil {
		return err
	}

	for _, tpeer := range tpeers {
		_, err := multiaddr.NewMultiaddr(tpeer)
		if err != nil {
			return fmt.Errorf("cmd: while parsing '%s' with peer addr '%s': %w", headersTrustedPeersFlag, tpeer, err)
		}
	}
	cfg.TrustedPeers = append(cfg.TrustedPeers, tpeers...)
	return nil
}

// TrustedHashFlags returns a set of flags related to configuring a `TrustedHash`.
func TrustedHashFlags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.String(
		headersTrustedHashFlag,
		"",
		"Hex encoded header hash. Used to subjectively initialize header synchronization",
	)
	return flags
}

// ParseTrustedHashFlags parses Header package flags from the given cmd and saves them to the
// passed config.
func ParseTrustedHashFlags(
	cmd *cobra.Command,
	cfg *Config,
) error {
	hash := cmd.Flag(headersTrustedHashFlag).Value.String()
	if hash != "" {
		_, err := hex.DecodeString(hash)
		if err != nil {
			return fmt.Errorf("cmd: while parsing '%s': %w", headersTrustedHashFlag, err)
		}

		cfg.TrustedHash = hash
	}
	return nil
}
