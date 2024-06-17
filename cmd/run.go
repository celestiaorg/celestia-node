package cmd

import (
	"os/signal"
	"syscall"

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

			config := NodeConfig(ctx)
			node, err := NewRunner(&config)
			if err != nil {
				return err
			}

			err = node.Init(ctx)
			if err != nil {
				return err
			}

			ctx, cancel := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()
			err = node.Start(ctx)
			if err != nil {
				return err
			}
			<-ctx.Done()
			cancel() // ensure we stop reading more signals for start context

			ctx, cancel = signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()
			return node.Stop(ctx)
		},
	}
	// Apply each passed option to the command
	for _, option := range options {
		option(cmd)
	}
	return cmd
}
