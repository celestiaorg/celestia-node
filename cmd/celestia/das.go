package main

import "github.com/spf13/cobra"

func init() {
	dasCmd.AddCommand(samplingStatsCmd)
}

var dasCmd = &cobra.Command{
	Use:   "das [command]",
	Short: "Allows to interact with the Daser via JSON-RPC",
	Args:  cobra.NoArgs,
}

var samplingStatsCmd = &cobra.Command{
	Use:   "sampling-stats",
	Short: "Returns the current statistics over the DA sampling process",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := rpcClient(cmd.Context())
		if err != nil {
			return err
		}
		stats, err := client.DAS.SamplingStats(cmd.Context())
		return printOutput(stats, err, nil)
	},
}
