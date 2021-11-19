package cmd

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os/signal"
	"syscall"

	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/node"
)

// Start constructs a CLI command to start Celestia Node daemon of the given type 'tp'.
// It is meant to be used a subcommand and also receive persistent flag name for repository path.
func Start(repoName string, tp node.Type) *cobra.Command {
	if !tp.IsValid() {
		panic("cmd: Start: invalid Node Type")
	}
	if len(repoName) == 0 && tp != node.Dev { // repository path not necessary for **DEV MODE**
		panic("parent command must specify a persistent flag name for repository path")
	}

	const cfgAddress = "core.remote"
	const loglevel = "log.level"
	cmd := &cobra.Command{
		Use:          "start",
		Short:        "Starts Node daemon. First stopping signal gracefully stops the Node and second terminates it.",
		Aliases:      []string{"run", "daemon"},
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			level, err := logging.LevelFromString(cmd.Flag(loglevel).Value.String())
			if err != nil {
				return fmt.Errorf("while setting log level: %w", err)
			}
			logging.SetAllLoggers(level)

			// **DEV MODE** needs separate configuration
			if tp == node.Dev {
				repo := node.NewMemRepository()
				cfg := node.DefaultConfig(tp)
				if err := repo.PutConfig(cfg); err != nil {
					return err
				}

				return start(cmd, tp, repo)
			}

			repoPath := cmd.Flag(repoName).Value.String()

			repo, err := node.Open(repoPath, tp)
			if err != nil {
				return err
			}

			address := cmd.Flag(cfgAddress).Value.String()
			opts := make([]node.Options, 0)
			if address != "" {
				protocol, ip, err := parseAddress(address)
				if err != nil {
					return err
				}
				opts = append(opts, node.WithRemoteClient(protocol, ip))
			}

			return start(cmd, tp, repo, opts...)
		},
	}

	cmd.Flags().String(cfgAddress, "", "Indicates node to connect to the given remote core node")
	cmd.Flags().String(
		loglevel,
		"info",
		"DEBUG, INFO, WARN, ERROR, DPANIC, PANIC, FATAL, and\n// their lower-case forms.",
	)
	return cmd
}

func start(cmd *cobra.Command, tp node.Type, repo node.Repository, opts ...node.Options) error {
	nd, err := node.New(tp, repo, opts...)
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

	return repo.Close()
}

// parseAddress parses the given address of the remote core node
// and checks if it configures correctly
func parseAddress(address string) (string, string, error) {
	u, err := url.Parse(address)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", "", err
	}

	if _, port, err := net.SplitHostPort(u.Host); err != nil || port == "" {
		return "", "", errors.New("incorrect address provided for Remote Core")
	}

	return u.Scheme, u.Host, nil
}
