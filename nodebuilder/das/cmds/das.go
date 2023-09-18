package rpc

import (
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/cmd/celestia/internal"
)

func init() {
	DASCmd.AddCommand(samplingStatsCmd)

	DASCmd.PersistentFlags().StringVar(
		&internal.RequestURL,
		"url",
		"http://localhost:26658",
		"Request URL",
	)
}

var DASCmd = &cobra.Command{
	Use:               "das [command]",
	Short:             "Allows to interact with the Daser via JSON-RPC",
	Args:              cobra.NoArgs,
	PersistentPreRunE: internal.InitClient,
	PersistentPostRun: internal.CloseClient,
}

var samplingStatsCmd = &cobra.Command{
	Use:   "sampling-stats",
	Short: "Returns the current statistics over the DA sampling process",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		stats, err := internal.RPCClient.DAS.SamplingStats(cmd.Context())
		return internal.PrintOutput(stats, err, nil)
	},
}
