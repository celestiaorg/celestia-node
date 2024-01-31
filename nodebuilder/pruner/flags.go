package pruner

import (
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

const pruningFlag = "experimental-pruning"

func Flags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.Bool(pruningFlag, false, "EXPERIMENTAL: Enables pruning of blocks outside the pruning window.")

	return flags
}

func ParseFlags(cmd *cobra.Command, cfg *Config) {
	enabled := cmd.Flag(pruningFlag).Changed
	if enabled {
		cfg.EnableService = true
	}
}
