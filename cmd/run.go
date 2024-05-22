package cmd

import (
	"github.com/spf13/cobra"
)

// Run constructs a CLI command to run Celestia Node daemon of any type with the given flags.
func Run(options ...func(*cobra.Command)) *cobra.Command {
	cmd := &cobra.Command{
		Use: "run",
		Short: `Run will both initialize and start a Node daemon. First stopping signal gracefully stops the Node.
		and second terminates it`,
		Aliases:      []string{"daemon"},
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			ctx := cmd.Context()

			node, err := NewRunner(ctx)
			if err != nil {
				return err
			}

			err = node.Init(ctx)
			if err != nil {
				return err
			}

			go func() {
				ctx := node.Start(ctx)
				<-ctx.Done()
				node.Stop(ctx)
			}()

			err = <-node.Errors()
			return err
		},
	}
	// Apply each passed option to the command
	for _, option := range options {
		option(cmd)
	}
	return cmd
}
