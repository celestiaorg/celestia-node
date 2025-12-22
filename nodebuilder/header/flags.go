package header

import (
	"fmt"

	"github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var headersTrustedPeersFlag = "headers.trusted-peers"

// Flags gives a set of hardcoded Header package flags.
func Flags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.AddFlagSet(TrustedPeersFlags())
	flags.AddFlagSet(SyncFromFlags())
	return flags
}

// ParseFlags parses Header package flags from the given cmd and applies them to the passed config.
func ParseFlags(cmd *cobra.Command, cfg *Config) (err error) {
	err = ParseTrustedPeerFlags(cmd, cfg)
	if err != nil {
		return err
	}
	return ParseSyncFromFlags(cmd, cfg)
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

func SyncFromFlags() *flag.FlagSet {
	flags := &flag.FlagSet{}
	flags.String(
		"header.sync-from-hash",
		"",
		"Hex-encoded hash of the header to start syncing from.",
	)
	flags.Uint64(
		"header.sync-from-height",
		0,
		"Height of the header to start syncing from.",
	)
	return flags
}

func ParseSyncFromFlags(cmd *cobra.Command, cfg *Config) error {
	hash, err := cmd.Flags().GetString("header.sync-from-hash")
	if err != nil {
		return err
	}
	if hash != "" {
		cfg.Syncer.SyncFromHash = hash
	}

	height, err := cmd.Flags().GetUint64("header.sync-from-height")
	if err != nil {
		return err
	}
	if height != 0 {
		cfg.Syncer.SyncFromHeight = height
	}
	return nil
}
