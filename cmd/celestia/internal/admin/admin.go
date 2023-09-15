package admin

import (
	"errors"
	"strings"

	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/cmd/celestia/internal"
)

func init() {
	NodeCmd.AddCommand(nodeInfoCmd, logCmd, verifyCmd, authCmd)

	NodeCmd.PersistentFlags().StringVar(
		&internal.RequestURL,
		"url",
		"http://localhost:26658",
		"Request URL",
	)
}

var NodeCmd = &cobra.Command{
	Use:               "node [command]",
	Short:             "Allows administrating running node.",
	Args:              cobra.NoArgs,
	PersistentPreRunE: internal.InitClient,
}

var nodeInfoCmd = &cobra.Command{
	Use:   "info",
	Args:  cobra.NoArgs,
	Short: "Returns administrative information about the node.",
	RunE: func(c *cobra.Command, args []string) error {
		info, err := internal.RPCClient.Node.Info(c.Context())
		return internal.PrintOutput(info, err, nil)
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
		for _, ll := range args {
			params := strings.Split(ll, ":")
			if len(params) != 2 {
				return errors.New("cmd: log-level arg must be in form <module>:<level>," +
					"e.g. pubsub:debug")
			}

			if err := internal.RPCClient.Node.LogLevelSet(c.Context(), params[0], params[1]); err != nil {
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
		perms, err := internal.RPCClient.Node.AuthVerify(c.Context(), args[0])
		return internal.PrintOutput(perms, err, nil)
	},
}

var authCmd = &cobra.Command{
	Use:   "set-permissions",
	Args:  cobra.MinimumNArgs(1),
	Short: "Signs and returns a new token with the given permissions.",
	RunE: func(c *cobra.Command, args []string) error {
		perms := make([]auth.Permission, len(args))
		for i, p := range args {
			perms[i] = (auth.Permission)(p)
		}

		result, err := internal.RPCClient.Node.AuthNew(c.Context(), perms)
		return internal.PrintOutput(result, err, nil)
	},
}
