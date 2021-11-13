package cmd

import (
	"net/url"

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

	const cfgAddress = "address"
	const cfgName = "config"
	cmd := &cobra.Command{
		Use:  "init",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgF := cmd.Flag(cfgName).Value
			repo := cmd.Flag(repoName).Value.String()

			if len(cfgF.String()) != 0 {
				cfg, err := node.LoadConfig(cfgF.String())
				if err != nil {
					return err
				}

				return node.InitWith(repo, tp, cfg)
			}

			u, err := url.Parse(cmd.Flag(cfgAddress).Value.String())
			opts := make([]node.Options, 0)

			if err == nil && u.Scheme != "" && u.Host != "" {
				opts = append(opts, node.WithRemoteClient(u.Scheme, u.Host))
			}

			return node.Init(repo, tp, opts...)
		},
	}
	cmd.Flags().StringP(cfgName, "c", "", "Path to a customized Config")
	cmd.Flags().String(cfgAddress, "", "Address of a remote core to connect with")
	return cmd
}
