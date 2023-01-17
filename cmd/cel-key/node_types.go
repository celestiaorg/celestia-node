package main

import (
	"errors"
	"fmt"
	"strings"

	sdkflags "github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

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
			"~/.celestia-light-mynetwork if --p2p.network MyNetwork is passed).")

	return flags
}

func ParseDirectoryFlags(cmd *cobra.Command) error {
	// if keyring-dir is explicitly set, use it
	if cmd.Flags().Changed(sdkflags.FlagKeyringDir) {
		return nil
	}

	nodeType := cmd.Flag(nodeDirKey).Value.String()
	if nodeType == "" {
		return errors.New("no node type provided")
	}

	network := cmd.Flag(networkKey).Value.String()
	if net, err := p2p.Network(network).Validate(); err == nil {
		network = string(net)
	} else {
		fmt.Println("WARNING: unknown network specified: ", network)
	}
	switch nodeType {
	case "bridge", "full", "light":
		keyPath := fmt.Sprintf("~/.celestia-%s-%s/keys", nodeType, strings.ToLower(network))
		fmt.Println("using directory: ", keyPath)
		if err := cmd.Flags().Set(sdkflags.FlagKeyringDir, keyPath); err != nil {
			return err
		}
	default:
		return fmt.Errorf("node type %s is not valid. Provided node types: (bridge || full || light)",
			nodeType)
	}
	return nil
}
