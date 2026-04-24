package cmd

import (
	"errors"
	"strings"

	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/spf13/cobra"

	cmdnode "github.com/celestiaorg/celestia-node/cmd"
)

func init() {
	authCmd.Flags().Duration("ttl", 0, "Set a Time-to-live (TTL) for the token")
	Cmd.AddCommand(nodeInfoCmd, logCmd, verifyCmd, authCmd)
}

var Cmd = &cobra.Command{
	Use:               "node [command]",
	Short:             "Allows administrating running node.",
	Args:              cobra.NoArgs,
	PersistentPreRunE: cmdnode.InitClient,
}

var nodeInfoCmd = &cobra.Command{
	Use:   "info",
	Args:  cobra.NoArgs,
	Short: "Returns administrative information about the node.",
	RunE: func(c *cobra.Command, _ []string) error {
		client, err := cmdnode.ParseClientFromCtx(c.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		info, err := client.Node.Info(c.Context())
		return cmdnode.PrintOutput(info, err, nil)
	},
}

var logCmd = &cobra.Command{
	Use:   "log-level",
	Args:  cobra.MinimumNArgs(1),
	Short: "Sets log level for module.",
	Long: "Allows to set log level for module to in format <module>:<level>" +
		"`DEBUG, INFO, WARN, ERROR, DPANIC, PANIC, FATAL and their lower-case forms`.\n" +
		"To set all modules to a particular level `*:<log.level>` should be passed",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(c.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		for _, ll := range args {
			params := strings.Split(ll, ":")
			if len(params) != 2 {
				return errors.New("cmd: log-level arg must be in form <module>:<level>," +
					"e.g. pubsub:debug")
			}

			if err := client.Node.LogLevelSet(c.Context(), params[0], params[1]); err != nil {
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
		client, err := cmdnode.ParseClientFromCtx(c.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		perms, err := client.Node.AuthVerify(c.Context(), args[0])
		return cmdnode.PrintOutput(perms, err, nil)
	},
}

var authCmd = &cobra.Command{
	Use:   "set-permissions",
	Args:  cobra.MinimumNArgs(1),
	Short: "Signs and returns a new token with the given permissions.",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmdnode.ParseClientFromCtx(cmd.Context())
		if err != nil {
			return err
		}
		defer client.Close()

		perms := make([]auth.Permission, len(args))
		for i, p := range args {
			perms[i] = (auth.Permission)(p)
		}

		ttl, err := cmd.Flags().GetDuration("ttl")
		if err != nil {
			return err
		}
		if ttl != 0 {
			result, err := client.Node.AuthNewWithExpiry(cmd.Context(), perms, ttl)
			return cmdnode.PrintOutput(result, err, nil)
		}

		result, err := client.Node.AuthNew(cmd.Context(), perms)
		return cmdnode.PrintOutput(result, err, nil)
	},
}
