package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/node"
)

// Init constructs a CLI command to initialize Celestia Node of the given type 'tp'.
// It is meant to be used a subcommand and also receive persistent flag name for store path.
func Init(storeFlagName string, tp node.Type) *cobra.Command {
	if !tp.IsValid() {
		panic("cmd: Init: invalid Node Type")
	}
	if len(storeFlagName) == 0 {
		panic("parent command must specify a persistent flag name for store path")
	}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialization for Celestia Node. Passed flags have persisted effect.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			storePath := cmd.Flag(storeFlagName).Value.String()
			if storePath == "" {
				return fmt.Errorf("store path must be specified")
			}

			var opts []node.Option

			nodeConfig := cmd.Flag(nodeConfigFlag.Name).Value.String()
			if nodeConfig != "" {
				cfg, err := node.LoadConfig(nodeConfig)
				if err != nil {
					return err
				}

				opts = append(opts, node.WithConfig(cfg))
			}

			trustedHash := cmd.Flag(trustedHashFlag.Name).Value.String()
			if trustedHash != "" {
				opts = append(opts, node.WithTrustedHash(trustedHash))
			}

			trustedPeer := cmd.Flag(trustedPeerFlag.Name).Value.String()
			if trustedPeer != "" {
				opts = append(opts, node.WithTrustedPeer(trustedPeer))
			}

			coreRemote := cmd.Flag(coreRemoteFlag.Name).Value.String()
			if coreRemote != "" {
				protocol, ip, err := parseAddress(coreRemote)
				if err != nil {
					return err
				}
				opts = append(opts, node.WithRemoteCore(protocol, ip))
			}

			mutualPeersStr := cmd.Flag(mutualPeersFlag.Name).Value.String()
			if mutualPeersStr != "" {
				mutualPeers := strings.Split(mutualPeersStr, ",")
				if len(mutualPeers) == 0 {
					return fmt.Errorf("%s flag is passed but no addresses were given", mutualPeersFlag.Name)
				}

				opts = append(opts, node.WithMutualPeers(mutualPeers))
			}

			return node.Init(storePath, tp, opts...)
		},
	}
	for _, flag := range configFlags {
		cmd.Flags().String(flag.Name, flag.DefValue, flag.Usage)
	}
	return cmd
}
