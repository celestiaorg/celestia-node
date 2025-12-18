package pruner

import (
	"time"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

const (
	archivalFlag = "archival"
	windowFlag   = "prune.window"
)

func Flags() *flag.FlagSet {
	flags := &flag.FlagSet{}

	flags.Bool(archivalFlag, false, "Enables archival mode, which disables pruning and enables the "+
		"storage of all blocks.")
	flags.Duration(windowFlag, 0, "Sets the time window for which the node will keep data squares (ODS). "+
		"Defaults to 0 (default config value used).")

	return flags
}

func ParseFlags(cmd *cobra.Command, tp node.Type) fx.Option {
	archivalChanged := cmd.Flag(archivalFlag).Changed
	if archivalChanged {
		if tp != node.Full && tp != node.Bridge {
			log.Fatal("Archival mode is only supported for Full and Bridge nodes")
		}
		log.Info("ARCHIVAL MODE ENABLED. All blocks will be synced and stored, however archival blocks will " +
			"be trimmed after the storage window. Expect to see pruning logs as the pruner will still run to trim")
		return fx.Replace(&Config{
			EnableService: false,
		})
	}

	log.Info("PRUNING MODE ENABLED. Node will prune blocks to save space.")

	cfg := DefaultConfig()
	window := cmd.Flag(windowFlag).Value.String()
	if window != "0s" {
		dur, err := time.ParseDuration(window)
		if err != nil {
			log.Fatal(err)
		}
		cfg.StorageWindow = dur
	}

	return fx.Replace(cfg)
}
