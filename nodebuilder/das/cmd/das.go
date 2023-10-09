package cmd

import (
	"github.com/spf13/cobra"

	cmdnode "github.com/celestiaorg/celestia-node/cmd"
)

func init() {
	Cmd.AddCommand(samplingStatsCmd)
}

var Cmd = &cobra.Command{
	Use:               "das [command]",
	Short:             "Allows to interact with the Daser via JSON-RPC",
	Args:              cobra.NoArgs,
	PersistentPreRunE: cmdnode.InitClient,
}

var samplingStatsCmd = &cobra.Command{
	Use:   "sampling-stats",
	Short: "Returns the current statistics over the DA sampling process",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		stats, err := client.DAS.SamplingStats(cmd.Context())
		return cmdnode.PrintOutput(stats, err, nil)
	},
}
