package main

import (
	"errors"
	"fmt"
	"strings"

	sdkflags "github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/nodebuilder"
	nodemod "github.com/celestiaorg/celestia-node/nodebuilder/node"
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

	nodeTypeStr := cmd.Flag(nodeDirKey).Value.String()
	nodeType := nodemod.ParseType(nodeTypeStr)
	if !nodeType.IsValid() {
		return errors.New("no or invalid node type provided")
	}

	network, err := p2p.ParseNetwork(cmd)
	if err != nil {
		return err
	}

	path, err := nodebuilder.DefaultNodeStorePath(nodeType, network)
	if err != nil {
		return err
	}

	keyPath := fmt.Sprintf("%s/keys", path)
	fmt.Println("using directory: ", keyPath)
	if err := cmd.Flags().Set(sdkflags.FlagKeyringDir, keyPath); err != nil {
		return err
	}

	return nil
}
