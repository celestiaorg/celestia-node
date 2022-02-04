package cmd

import (
	"fmt"

	"github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/node"
)

var (
	headersTrustedPeerFlag = "headers.trusted-peer"
)

// HeadersFlags gives a set of hardcoded Header package flags.
func TrustedPeerFlags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.String(
		headersTrustedPeerFlag,
		"",
		"Multiaddr of a reliable peer to fetch headers from. (Format: multiformats.io/multiaddr)",
	)

	return flags
}

// ParseHeadersFlags parses Header package flags from the given cmd and applies values to Env.
func ParseTrustedPeerFlags(cmd *cobra.Command, env *Env) error {
	tpeer := cmd.Flag(headersTrustedPeerFlag).Value.String()
	if tpeer != "" {
		_, err := multiaddr.NewMultiaddr(tpeer)
		if err != nil {
			return fmt.Errorf("cmd: while parsing '%s': %w", headersTrustedPeerFlag, err)
		}

		env.AddOptions(node.WithTrustedPeer(tpeer))
	}

	return nil
}
