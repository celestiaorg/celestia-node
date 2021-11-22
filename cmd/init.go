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

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialization for Celestia Node. Passed flags have persisted effect.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := cmd.Flag("repository").Value.String() // TODO @renaynay: create persistentFlags var

			nodeConfig := cmd.Flag(nodeConfigFlag.Name).Value.String()
			if nodeConfig != "" {
				cfg, err := node.LoadConfig(nodeConfig)
				if err != nil {
					return err
				}

				return node.InitWith(repo, tp, cfg)
			}

			var opts []node.Option

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

			return node.Init(repo, tp, opts...)
		},
	}
	for _, flag := range configFlags {
		cmd.Flags().String(flag.Name, flag.DefValue, flag.Usage)
	}
	return cmd
}
