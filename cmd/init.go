package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/node"
)

// Init constructs a CLI command to initialize Celestia Node of the given type 'tp'.
// It is meant to be used a subcommand and also receive persistent flag name for repository path.
func Init(repoFlagName string, tp node.Type) *cobra.Command {
	if !tp.IsValid() {
		panic("cmd: Init: invalid Node Type")
	}
	if len(repoFlagName) == 0 {
		panic("parent command must specify a persistent flag name for repository path")
	}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialization for Celestia Node. Passed flags have persisted effect.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath := cmd.Flag(repoFlagName).Value.String()
			if repoPath == "" {
				return fmt.Errorf("repository path must be specified")
			}

			nodeConfig := cmd.Flag(nodeConfigFlag.Name).Value.String()
			if nodeConfig != "" {
				cfg, err := node.LoadConfig(nodeConfig)
				if err != nil {
					return err
				}

				return node.InitWith(repoPath, tp, cfg)
			}

			var opts []node.Option

			p2pKey := cmd.Flag(p2pKeyFlag.Name).Value.String()
			if p2pKey != "" {
				opts = append(opts, node.WithP2PKeyStr(p2pKey))
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

			return node.Init(repoPath, tp, opts...)
		},
	}
	for _, flag := range configFlags {
		cmd.Flags().String(flag.Name, flag.DefValue, flag.Usage)
	}
	return cmd
}
