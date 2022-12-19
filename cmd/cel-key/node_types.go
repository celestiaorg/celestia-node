package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	sdkflags "github.com/cosmos/cosmos-sdk/client/flags"

	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

var (
	nodeDirKey = "node.type"
	networkKey = "p2p.network"
)

func DirectoryFlags() *flag.FlagSet {
	flags := &flag.FlagSet{}
	defaultNetwork := string(p2p.DefaultNetwork)

	flags.String(
		nodeDirKey,
		"",
		"Sets key utility to use the node type's directory (e.g. "+
			"~/.celestia-light-"+strings.ToLower(defaultNetwork)+" if --node.type light is passed).")
	flags.String(
		networkKey,
		defaultNetwork,
		"Sets key utility to use the node network's directory (e.g. "+
			"~/.celestia-light-mynetwork if --node.network MyNetwork is passed).")

	return flags
}

func ParseDirectoryFlags(cmd *cobra.Command) error {
	nodeType := cmd.Flag(nodeDirKey).Value.String()
	if nodeType == "" {
		return errors.New("no node type provided")
	}

	network := cmd.Flag(networkKey).Value.String()
	switch nodeType {
	case "bridge", "full", "light":
		// check to see if an alias has been provided
		if net, ok := p2p.NetworkAliases[network]; ok {
			network = string(net)
		}
		keyPath := fmt.Sprintf("~/.celestia-%s-%s/keys", nodeType, strings.ToLower(network))
		if err := cmd.Flags().Set(sdkflags.FlagKeyringDir, keyPath); err != nil {
			return err
		}
	default:
		return fmt.Errorf("node type %s is not valid. Provided node types: (bridge || full || light)",
			nodeType)
	}
	return nil
}
