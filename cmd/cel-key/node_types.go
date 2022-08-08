package main

import (
	"fmt"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	sdkflags "github.com/cosmos/cosmos-sdk/client/flags"
)

var (
	nodeDirKey = "node.type"

	bridgeDir = "~/.celestia-bridge/keys"
	fullDir   = "~/.celestia-full/keys"
	lightDir  = "~/.celestia-light/keys"
)

func DirectoryFlags() *flag.FlagSet {
	flags := &flag.FlagSet{}
	flags.String(nodeDirKey, "", "Sets key utility to use the node type's directory (e.g. "+
		"~/.celestia-light if --node.type light is passed).")

	return flags
}

func ParseDirectoryFlags(cmd *cobra.Command) error {
	nodeType := cmd.Flag(nodeDirKey).Value.String()
	if nodeType == "" {
		return nil
	}

	switch nodeType {
	case "bridge":
		if err := cmd.Flags().Set(sdkflags.FlagKeyringDir, bridgeDir); err != nil {
			return err
		}
	case "full":
		if err := cmd.Flags().Set(sdkflags.FlagKeyringDir, fullDir); err != nil {
			return err
		}
	case "light":
		if err := cmd.Flags().Set(sdkflags.FlagKeyringDir, lightDir); err != nil {
			return err
		}
	default:
		return fmt.Errorf("node type %s is not valid. Provided node types: (bridge || full || light)",
			nodeType)
	}
	return nil
}
