package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/params"

	sdkflags "github.com/cosmos/cosmos-sdk/client/flags"
)

var (
	nodeDirKey     = "node.type"
	nodeNetworkKey = "node.network"
)

func DirectoryFlags() *flag.FlagSet {
	flags := &flag.FlagSet{}
	defaultNetwork := string(params.DefaultNetwork())

	flags.String(
		nodeDirKey,
		"",
		"Sets key utility to use the node type's directory (e.g. "+
			"~/.celestia-light-"+strings.ToLower(defaultNetwork)+" if --node.type light is passed).")
	flags.String(
		nodeNetworkKey,
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

	network := cmd.Flag(nodeNetworkKey).Value.String()
	switch nodeType {
	case "bridge", "full", "light":
		keyPath := fmt.Sprintf("~/.celestia-%s-%s", nodeType, strings.ToLower(network))
		if err := cmd.Flags().Set(sdkflags.FlagKeyringDir, keyPath); err != nil {
			return err
		}
	default:
		return fmt.Errorf("node type %s is not valid. Provided node types: (bridge || full || light)",
			nodeType)
	}
	return nil
}
