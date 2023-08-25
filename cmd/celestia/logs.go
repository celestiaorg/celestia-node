package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/cmd"
)

var logCmd = &cobra.Command{
	Use:  cmd.LogLevelFlag,
	Args: cobra.ExactArgs(1),
	Short: "Allows to set log level for all modules to " +
		"`DEBUG, INFO, WARN, ERROR, DPANIC, PANIC, FATAL and their lower-case forms`",

	RunE: func(c *cobra.Command, args []string) error {
		client, err := rpcClient(c.Context())
		if err != nil {
			return err
		}
		return client.Node.LogLevelSet(c.Context(), "*", args[0])
	},
}

var logModuleCmd = &cobra.Command{
	Use:   cmd.LogLevelModuleFlag,
	Args:  cobra.MinimumNArgs(1),
	Short: "Allows to set log level for a particular module in format <module>:<level>",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := rpcClient(c.Context())
		if err != nil {
			return err
		}
		for _, ll := range args {
			params := strings.Split(ll, ":")
			if len(params) != 2 {
				return fmt.Errorf("cmd: %s arg must be in form <module>:<level>,"+
					"e.g. pubsub:debug", cmd.LogLevelModuleFlag)
			}
			if err = client.Node.LogLevelSet(c.Context(), params[0], params[1]); err != nil {
				return err
			}
		}
		return nil
	},
}
