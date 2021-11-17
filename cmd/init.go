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
			return node.Init(repo, tp)
		},
	}
	cmd.Flags().StringP(cfgName, "c", "", "Path to a customized Config")
	return cmd
}
