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

	"github.com/celestiaorg/celestia-node/logs"
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

	cmd := &cobra.Command{
		Use: "start",
		Short: `
			Starts Node daemon. First stopping signal gracefully stops the Node and second terminates it.
			Passed actions have effect only ont this start and not persisted. 
		`,
		Aliases:      []string{"run", "daemon"},
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			level, err := logging.LevelFromString(cmd.Flag(loglevelFlag.Name).Value.String())
			if err != nil {
				return fmt.Errorf("while setting log level: %w", err)
			}
			logs.SetAllLoggers(level)

			// **DEV MODE** needs separate configuration
			if tp == node.Dev {
				repo := node.NewMemRepository()
				cfg := node.DefaultConfig(tp)
				if err := repo.PutConfig(cfg); err != nil {
					return err
				}

				return start(cmd, tp, repo)
			}

			var opts []node.Option

			trustedHash := cmd.Flag(trustedHashFlag.Name).Value.String()
			if trustedHash != "" {
				opts = append(opts, node.WithTrustedHash(trustedHash))
			}
			trustedPeer := cmd.Flag(trustedPeerFlag.Name).Value.String()
			if trustedPeer != "" {
				opts = append(opts, node.WithTrustedPeer(trustedPeer))
			}
			coreRemote := cmd.Flag(coreRemoteFlag.Name).Value.String()
			if coreRemote != "" {
				protocol, ip, err := parseAddress(coreRemote)
				if err != nil {
					return err
				}
				opts = append(opts, node.WithRemoteCore(protocol, ip))
			}
			nodeConfig := cmd.Flag(nodeConfigFlag.Name).Value.String()
			if nodeConfig != "" {
				cfg, err := node.LoadConfig(nodeConfig)
				if err != nil {
					return err
				}
				opts = append(opts, node.WithConfig(cfg))
			}

			repo, err := node.Open(repoName, tp)
			if err != nil {
				return err
			}

			return start(cmd, tp, repo, opts...)
		},
	}
	for _, flag := range configFlags {
		cmd.Flags().String(flag.Name, flag.DefValue, flag.Usage)
	}
	return cmd
}

func start(cmd *cobra.Command, tp node.Type, repo node.Repository, opts ...node.Option) error {
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
