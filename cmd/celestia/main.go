package main

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	cmdnode "github.com/celestiaorg/celestia-node/cmd"
)

// WithSubcommands returns the set of commands that require the full flagset.
func WithSubcommands() func(*cobra.Command, []*pflag.FlagSet) {
	return func(c *cobra.Command, flags []*pflag.FlagSet) {
		c.AddCommand(
			cmdnode.Init(flags...),
			cmdnode.Start(cmdnode.WithFlagSet(flags)),
		)
	}
}

// WithAuxiliarySubcommands returns the set of commands that require only the
// minimum flagset for node store determination.
func WithAuxiliarySubcommands() func(*cobra.Command, []*pflag.FlagSet) {
	return func(c *cobra.Command, flags []*pflag.FlagSet) {
		c.AddCommand(
			cmdnode.AuthCmd(flags...),
			cmdnode.ResetStore(flags...),
			cmdnode.RemoveConfigCmd(flags...),
			cmdnode.UpdateConfigCmd(flags...),
		)
	}
}

func init() {
	bridgeCmd := cmdnode.NewBridge(WithSubcommands(), WithAuxiliarySubcommands())
	lightCmd := cmdnode.NewLight(WithSubcommands(), WithAuxiliarySubcommands())
	fullCmd := cmdnode.NewFull(WithSubcommands(), WithAuxiliarySubcommands())
	rootCmd.AddCommand(
		bridgeCmd,
		lightCmd,
		fullCmd,
		docgenCmd,
		versionCmd,
	)
	rootCmd.SetHelpCommand(&cobra.Command{})
}

func main() {
	err := run()
	if err != nil {
		os.Exit(1)
	}
}

func run() error {
	return rootCmd.ExecuteContext(context.Background())
}

var rootCmd = &cobra.Command{
	Use: "celestia [  bridge  ||  full ||  light  ] [subcommand]",
	Short: `
	    ____      __          __  _
	  / ____/__  / /__  _____/ /_(_)___ _
	 / /   / _ \/ / _ \/ ___/ __/ / __  /
	/ /___/  __/ /  __(__  ) /_/ / /_/ /
	\____/\___/_/\___/____/\__/_/\__,_/
	`,
	Args: cobra.NoArgs,
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: false,
	},
}
