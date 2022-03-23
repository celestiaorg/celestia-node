package cmd

import (
	"encoding/hex"
	"fmt"

	"github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/node"
)

var (
	headersTrustedHashFlag  = "headers.trusted-hash"
	headersTrustedPeersFlag = "headers.trusted-peers"
)

// HeadersFlags gives a set of hardcoded Header package flags.
func HeadersFlags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.AddFlagSet(TrustedPeersFlags())
	flags.AddFlagSet(TrustedHashFlags())

	return flags
}

// ParseHeadersFlags parses Header package flags from the given cmd and applies values to Env.
func ParseHeadersFlags(cmd *cobra.Command, env *Env) error {
	if err := ParseTrustedHashFlags(cmd, env); err != nil {
		return err
	}
	if err := ParseTrustedPeerFlags(cmd, env); err != nil {
		return err
	}

	return nil
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

// ParseTrustedPeerFlags parses Header package flags from the given cmd and applies values to Env.
func ParseTrustedPeerFlags(cmd *cobra.Command, env *Env) error {
	tpeers, err := cmd.Flags().GetStringSlice(headersTrustedPeersFlag)
	if err != nil {
		return err
	}

	for _, tpeer := range tpeers {
		_, err := multiaddr.NewMultiaddr(tpeer)
		if err != nil {
			return fmt.Errorf("cmd: while parsing '%s' with peer addr '%s': %w", headersTrustedPeersFlag, tpeer, err)
		}
		env.AddOptions(node.WithTrustedPeer(tpeer))
	}

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

// ParseTrustedHashFlags parses Header package flags from the given cmd and applies values to Env.
func ParseTrustedHashFlags(cmd *cobra.Command, env *Env) error {
	hash := cmd.Flag(headersTrustedHashFlag).Value.String()
	if hash != "" {
		_, err := hex.DecodeString(hash)
		if err != nil {
			return fmt.Errorf("cmd: while parsing '%s': %w", headersTrustedHashFlag, err)
		}

		env.AddOptions(node.WithTrustedHash(hash))
	}

	return nil
}
