package pruner

import (
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"go.uber.org/fx"

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
	return nil
}
