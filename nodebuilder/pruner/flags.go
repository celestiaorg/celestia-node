package pruner

import (
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

const (
	archivalFlag = "archival"
)

func Flags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.Bool(archivalFlag, false, "Enables archival mode, which disables pruning and enables the "+
		"storage of all blocks.")

	return flags
}

func ParseFlags(cmd *cobra.Command, cfg *Config, tp node.Type) {
	archivalChanged := cmd.Flag(archivalFlag).Changed

	// Validate archival flag usage early to prevent invalid configurations
	if archivalChanged {
		if tp != node.Full && tp != node.Bridge {
			log.Fatal("Archival mode is only supported for Full and Bridge nodes")
		}

		cfg.EnableService = false
		log.Info("ARCHIVAL MODE ENABLED. All blocks will be synced and stored.")
		return
	}

	if cfg.EnableService {
		log.Info("PRUNING MODE ENABLED. Node will prune blocks to save space.")
		return
	}

	// If node operator does not explicitly pass `--archival` flag and pruner service is disabled, disallow
	log.Fatal("ARCHIVAL MODE ENABLED BY ACCIDENT!!! Please explicitly pass `--archival` to enable archival mode!")
}
