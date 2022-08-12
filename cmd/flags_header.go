package cmd

import (
	"context"
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
func ParseHeadersFlags(ctx context.Context, cmd *cobra.Command) (context.Context, error) {
	if ctx, err := ParseTrustedHashFlags(ctx, cmd); err != nil {
		return ctx, err
	}
	if ctx, err := ParseTrustedPeerFlags(ctx, cmd); err != nil {
		return ctx, err
	}

	return ctx, nil
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
func ParseTrustedPeerFlags(ctx context.Context, cmd *cobra.Command) (context.Context, error) {
	tpeers, err := cmd.Flags().GetStringSlice(headersTrustedPeersFlag)
	if err != nil {
		return ctx, err
	}

	for _, tpeer := range tpeers {
		_, err := multiaddr.NewMultiaddr(tpeer)
		if err != nil {
			return ctx, fmt.Errorf("cmd: while parsing '%s' with peer addr '%s': %w", headersTrustedPeersFlag, tpeer, err)
		}
	}

	ctx = WithNodeOptions(ctx, node.WithTrustedPeers(tpeers...))

	return ctx, nil
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
func ParseTrustedHashFlags(ctx context.Context, cmd *cobra.Command) (context.Context, error) {
	hash := cmd.Flag(headersTrustedHashFlag).Value.String()
	if hash != "" {
		_, err := hex.DecodeString(hash)
		if err != nil {
			return ctx, fmt.Errorf("cmd: while parsing '%s': %w", headersTrustedHashFlag, err)
		}

		ctx = WithNodeOptions(ctx, node.WithTrustedHash(hash))
	}

	return ctx, nil
}
