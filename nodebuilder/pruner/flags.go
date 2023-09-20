package pruner

import (
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var (
	archivalModeFlag = "archival-mode"
)

func Flags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.Bool(
		archivalModeFlag,
		false,
		"Enables archival mode. When enabled, the node will not prune old data. "+
			"Note that this flag is only applicable to full and bridge nodes.",
	)
	return flags
}

func ParseFlags(
	cmd *cobra.Command,
	cfg *Config,
) {
	archivalMode, err := cmd.Flags().GetBool(archivalModeFlag)
	if err != nil {
		panic(err)
	}

	if cmd.Flags().Changed(archivalModeFlag) {
		cfg.PruningEnabled = !archivalMode
	}
}
