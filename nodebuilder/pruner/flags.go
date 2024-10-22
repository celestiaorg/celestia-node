package pruner

import (
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

const (
	pruningFlag  = "experimental-pruning"
	archivalFlag = "archival"
)

func Flags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.Bool(pruningFlag, false, "EXPERIMENTAL: Enables pruning of blocks outside the pruning window.")
	flags.Bool(archivalFlag, false, "Enables archival mode, which disables pruning and enables the storage of all blocks.")

	return flags
}

func ParseFlags(cmd *cobra.Command, cfg *Config) {
	cfg.EnableService = cmd.Flag(pruningFlag).Changed
	if cfg.EnableService && cmd.Flag(archivalFlag).Changed {
		log.Fatal("Cannot enable both pruning and archival modes")
	}
}
