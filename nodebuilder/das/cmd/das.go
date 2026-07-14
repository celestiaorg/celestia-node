package cmd

import (
	"strconv"

	"github.com/spf13/cobra"

	cmdnode "github.com/celestiaorg/celestia-node/cmd"
)

func init() {
	Cmd.AddCommand(samplingStatsCmd)
	Cmd.AddCommand(resetSampledToCmd)
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
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		stats, err := client.DAS.SamplingStats(cmd.Context())
		return cmdnode.PrintOutput(stats, err, nil)
	},
}

var resetSampledToCmd = &cobra.Command{
	Use:   "reset-sampled-to [height]",
	Short: "Resets the DAS sampling position to a height so sampling resumes from it upward",
	Long: "Moves the DAS sampling position to the given height. Heights below it are treated as " +
		"sampled and previously failed heights are cleared; sampling then proceeds from this " +
		"height up to the network head. The new position is persisted across restarts.",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		height, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return err
		}

		err = client.DAS.ResetSampledTo(cmd.Context(), height)
		return cmdnode.PrintOutput(nil, err, nil)
	},
}
