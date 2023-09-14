package main

import (
	"context"
	"errors"
	"strings"

	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/spf13/cobra"
)

func init() {
	nodeCmd.AddCommand(nodeInfoCmd, logCmd, verifyCmd, authCmd)
	rootCmd.AddCommand(nodeCmd)
}

var nodeCmd = &cobra.Command{
	Use:   "node [command]",
	Short: "Allows to interact with the \"administrative\" node.",
	Args:  cobra.NoArgs,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		rpcClient, err := newRPCClient(cmd.Context())
		if err != nil {
			return err
		}

		ctx := context.WithValue(cmd.Context(), rpcClientKey{}, rpcClient)
		cmd.SetContext(ctx)
		return nil
	},
}

var nodeInfoCmd = &cobra.Command{
	Use:   "info",
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
	Use:  "log-level",
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
				return errors.New("cmd: log-level arg must be in form <module>:<level>," +
					"e.g. pubsub:debug")
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
	Use:   "set-permissions",
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
