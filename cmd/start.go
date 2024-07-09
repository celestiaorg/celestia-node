package cmd

import (
	"errors"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-app/v2/app"
	"github.com/celestiaorg/celestia-app/v2/app/encoding"

	"github.com/celestiaorg/celestia-node/nodebuilder"
)

// Start constructs a CLI command to start Celestia Node daemon of any type with the given flags.
func Start(options ...func(*cobra.Command)) *cobra.Command {
	cmd := &cobra.Command{
		Use: "start",
		Short: `Starts Node daemon. First stopping signal gracefully stops the Node and second terminates it.
Options passed on start override configuration options only on start and are not persisted in config.`,
		Aliases:      []string{"run", "daemon"},
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			ctx := cmd.Context()

			// override config with all modifiers passed on start
			cfg := NodeConfig(ctx)

			storePath := StorePath(ctx)
			keysPath := filepath.Join(storePath, "keys")

			// construct ring
			// TODO @renaynay: Include option for setting custom `userInput` parameter with
			//  implementation of https://github.com/celestiaorg/celestia-node/issues/415.
			encConf := encoding.MakeConfig(app.ModuleEncodingRegisters...)
			ring, err := keyring.New(app.Name, cfg.State.DefaultBackendName, keysPath, os.Stdin, encConf.Codec)
			if err != nil {
				return err
			}

			store, err := nodebuilder.OpenStore(storePath, ring)
			if err != nil {
				return err
			}
			defer func() {
				err = errors.Join(err, store.Close())
			}()

			nd, err := nodebuilder.NewWithConfig(NodeType(ctx), Network(ctx), store, &cfg, NodeOptions(ctx)...)
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
			return nd.Stop(ctx)
		},
	}
	// Apply each passed option to the command
	for _, option := range options {
		option(cmd)
	}
	return cmd
}
