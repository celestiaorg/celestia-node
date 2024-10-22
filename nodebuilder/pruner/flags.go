package pruner

import (
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
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

func ParseFlags(cmd *cobra.Command, cfg *Config, tp node.Type) {
	pruningChanged := cmd.Flag(pruningFlag).Changed
	archivalChanged := cmd.Flag(archivalFlag).Changed

	cfg.EnableService = pruningChanged

	// Validate archival flag usage early to prevent invalid configurations
	if archivalChanged {
		if tp != node.Full && tp != node.Bridge {
			log.Fatal("Archival mode is only supported for Full and Bridge nodes")
		}
		if cfg.EnableService {
			log.Fatal("Cannot enable both pruning and archival modes")
		}
	}

	// Add pruning flag deprecation warning
	if cfg.EnableService {
		log.Warn(`WARNING: --experimental-pruning flag will be removed in an upcoming release.
Pruning will become the default mode for all nodes.
If you want to retain history beyond the sampling window, please pass the --archival flag.`)
		return
	}

	// Warn the user if pruning is disabled and archival is not enabled for Full and Bridge nodes
	if !archivalChanged && (tp == node.Full || tp == node.Bridge) {
		log.Warn(`WARNING: Pruning is disabled.
Pruning will become the default mode for all nodes.
If you want to retain history beyond the sampling window, please pass the --archival flag.`)
	}
}
