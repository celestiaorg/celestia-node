package main

import (
	"fmt"
	"strings"

	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/cmd"
)

var nodeInfoCmd = &cobra.Command{
	Use:   "node.info",
	Args:  cobra.NoArgs,
	Short: "Returns administrative information about the node.",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := rpcClient(c.Context())
		if err != nil {
			return err
		}
		info, err := client.Node.Info(c.Context())
		return printOutput(info, err, nil)
	},
}

var logCmd = &cobra.Command{
	Use:  cmd.LogLevelFlag,
	Args: cobra.MinimumNArgs(1),
	Short: "Allows to set log level for module to in format <module>:<level>" +
		"`DEBUG, INFO, WARN, ERROR, DPANIC, PANIC, FATAL and their lower-case forms`.\n" +
		"To set all modules to a particular level `*:<log.level>` should be passed",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := rpcClient(c.Context())
		if err != nil {
			return err
		}

		for _, ll := range args {
			params := strings.Split(ll, ":")
			if len(params) != 2 {
				return fmt.Errorf("cmd: %s arg must be in form <module>:<level>,"+
					"e.g. pubsub:debug", cmd.LogLevelFlag)
			}

			if err = client.Node.LogLevelSet(c.Context(), params[0], params[1]); err != nil {
				return err
			}
		}
		return nil
	},
}

var verifyCmd = &cobra.Command{
	Use:   "permissions",
	Args:  cobra.ExactArgs(1),
	Short: "Returns the permissions assigned to the given token.",

	RunE: func(c *cobra.Command, args []string) error {
		client, err := rpcClient(c.Context())
		if err != nil {
			return err
		}

		perms, err := client.Node.AuthVerify(c.Context(), args[0])
		return printOutput(perms, err, nil)
	},
}

var authCmd = &cobra.Command{
	Use:   "set.permissions",
	Args:  cobra.MinimumNArgs(1),
	Short: "Signs and returns a new token with the given permissions.",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := rpcClient(c.Context())
		if err != nil {
			return err
		}

		perms := make([]auth.Permission, len(args))
		for i, p := range args {
			perms[i] = (auth.Permission)(p)
		}

		result, err := client.Node.AuthNew(c.Context(), perms)
		return printOutput(result, err, nil)
	},
}
