package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/node"
)

// NewRootCmd returns an initiated celestia root command. The provided plugins
// will be installed into celestia-node.
func NewRootCmd(plugs ...node.Plugin) *cobra.Command {
	plugins := make([]string, len(plugs))
	for i, plug := range plugs {
		plugins[i] = fmt.Sprintf("with plugin: %s", plug.Name())
	}

	command := &cobra.Command{
		Use: "celestia [  bridge  ||  light  ] [subcommand]",
		Short: fmt.Sprintf(`
		  / ____/__  / /__  _____/ /_(_)___ _
		 / /   / _ \/ / _ \/ ___/ __/ / __  /
		/ /___/  __/ /  __(__  ) /_/ / /_/ /
		\____/\___/_/\___/____/\__/_/\__,_/
		%s
		`, strings.Join(plugins, "\n")),
		Args: cobra.NoArgs,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}
	command.AddCommand(
		NewBridgeCommand(plugs),
		NewLightCommand(plugs),
		NewFullCommand(plugs),
		versionCmd,
	)
	command.SetHelpCommand(&cobra.Command{})
	return command
}
