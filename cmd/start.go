package cmd

import (
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/node"
)

// Start constructs a CLI command to start Celestia Node daemon of any type with the given flags.
func Start(fsets ...*flag.FlagSet) *cobra.Command {
	cmd := &cobra.Command{
		Use: "start",
		Short: `Starts Node daemon. First stopping signal gracefully stops the Node and second terminates it.
Options passed on start override configuration options only on start and are not persisted in config.`,
		Aliases:      []string{"run", "daemon"},
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			store, err := node.OpenStore(StorePath(ctx))
			if err != nil {
				return err
			}

			nd, err := node.New(NodeType(ctx), store, NodeOptions(ctx)...)
			if err != nil {
				return err
			}

			ctx, cancel := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()
			err = nd.Start(ctx)
			if err != nil {
				return err
			}

			<-ctx.Done()
			cancel() // ensure we stop reading more signals for start context

			ctx, cancel = signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()
			err = nd.Stop(ctx)
			if err != nil {
				return err
			}

			return store.Close()
		},
	}
	for _, set := range fsets {
		cmd.Flags().AddFlagSet(set)
	}
	return cmd
}
