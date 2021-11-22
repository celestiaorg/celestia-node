package cmd

import (
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/node"
)

// Init constructs a CLI command to initialize Celestia Node of the given type 'tp'.
// It is meant to be used a subcommand and also receive persistent flag name for repository path.
func Init(repoName string, tp node.Type) *cobra.Command {
	if !tp.IsValid() {
		panic("cmd: Init: invalid Node Type")
	}
	if len(repoName) == 0 {
		panic("parent command must specify a persistent flag name for repository path")
	}

	const (
		nodeConfig  = "node.config"
		genesis     = "headers.genesis-hash"
		trustedPeer = "headers.trusted-peer"
		coreRemote  = "core.remote"
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialization for Celestia Node. Passed flags have persisted effect.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := cmd.Flag(repoName).Value.String()
			nodeConfig := cmd.Flag(nodeConfig).Value.String()

			if nodeConfig != "" {
				cfg, err := node.LoadConfig(nodeConfig)
				if err != nil {
					return err
				}

				return node.InitWith(repo, tp, cfg)
			}

			genesis := cmd.Flag(genesis).Value.String()
			trustedPeer := cmd.Flag(trustedPeer).Value.String()
			coreRemote := cmd.Flag(coreRemote).Value.String()

			var opts []node.Option
			if genesis != "" {
				opts = append(opts, node.WithGenesis(genesis))
			}
			if trustedPeer != "" {
				opts = append(opts, node.WithTrustedPeer(trustedPeer))
			}
			if coreRemote != "" {
				protocol, ip, err := parseAddress(coreRemote)
				if err != nil {
					return err
				}
				opts = append(opts, node.WithRemoteCore(protocol, ip))
			}

			return node.Init(repo, tp, opts...)
		},
	}

	cmd.Flags().StringP(nodeConfig, "c", "", "Path to a customized Config.")
	cmd.Flags().String(genesis, "", "Hex encoded block hash. Starting point for header synchronization.")
	cmd.Flags().String(trustedPeer, "", "Multiaddr of a reliable peer to fetch headers from.")
	cmd.Flags().String(coreRemote, "", "Indicates node to connect to the given remote core node.")
	return cmd
}
