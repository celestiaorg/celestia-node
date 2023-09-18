package cmd

import (
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/cmd/celestia/util"
)

var rpcClient *client.Client

func init() {
	DASCmd.AddCommand(samplingStatsCmd)
}

var DASCmd = &cobra.Command{
	Use:   "das [command]",
	Short: "Allows to interact with the Daser via JSON-RPC",
	Args:  cobra.NoArgs,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		rpcClient, err = client.NewClient(cmd.Context(), client.RequestURL, "")
		return err
	},
	PersistentPostRun: func(_ *cobra.Command, _ []string) {
		rpcClient.Close()
	},
}

var samplingStatsCmd = &cobra.Command{
	Use:   "sampling-stats",
	Short: "Returns the current statistics over the DA sampling process",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		stats, err := rpcClient.DAS.SamplingStats(cmd.Context())
		return util.PrintOutput(stats, err, nil)
	},
}
