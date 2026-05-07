package cmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	rpc "github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/api/rpc/perms"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	nodemod "github.com/celestiaorg/celestia-node/nodebuilder/node"
)

var (
	requestURL    string
	authTokenFlag string
	timeoutFlag   time.Duration
)

func RPCFlags() *flag.FlagSet {
	fset := &flag.FlagSet{}

	fset.StringVar(
		&requestURL,
		"url",
		"", // will try to load value from Config, which defines its own default url
		"External RPC endpoint URL. If not specified, uses the local node address loaded from the config file.",
	)

	fset.StringVar(
		&authTokenFlag,
		"token",
		"",
		"Authorization token that is used to access the RPC endpoint. If not specified, it will be loaded and built "+
			"from the root folder of the node",
	)

	fset.DurationVar(
		&timeoutFlag,
		"timeout",
		0,
		"Timeout for RPC requests (e.g. 30s, 1m)",
	)

	storeFlag := NodeFlags().Lookup(nodeStoreFlag)
	fset.AddFlag(storeFlag)
	return fset
}

func InitClient(cmd *cobra.Command, _ []string) error {
	if authTokenFlag == "" {
		rootErrMsg := "cant access the auth token"

		storePath, err := getStorePath(cmd)
		if err != nil {
			return fmt.Errorf("%s: %w", rootErrMsg, err)
		}

		cfg, err := nodebuilder.LoadConfig(filepath.Join(storePath, "config.toml"))
		if err != nil {
			return fmt.Errorf("%s: root directory was not specified: %w", rootErrMsg, err)
		}

		if requestURL == "" {
			requestURL = cfg.RPC.RequestURL()
		}

		// only get token if auth is not skipped
		if cfg.RPC.SkipAuth {
			authTokenFlag = "skip" // arbitrary value required
		} else {
			token, err := getToken(storePath)
			if err != nil {
				return fmt.Errorf("%s: %w", rootErrMsg, err)
			}

			authTokenFlag = token
		}
	}

	if timeoutFlag > 0 {
		// we don't cancel, because we want to keep this context alive outside the InitClient Function
		ctx, _ := context.WithTimeout(cmd.Context(), timeoutFlag) //nolint:govet
		cmd.SetContext(ctx)
	}

	client, err := rpc.NewClient(cmd.Context(), requestURL, authTokenFlag)
	if err != nil {
		return err
	}

	ctx := context.WithValue(cmd.Context(), rpcClientKey{}, client)
	cmd.SetContext(ctx)
	return nil
}

func getStorePath(cmd *cobra.Command) (string, error) {
	// if node store flag is set, use it
	if cmd.Flag(nodeStoreFlag).Changed {
		return cmd.Flag(nodeStoreFlag).Value.String(), nil
	}

	// try to detect a running node
	path, err := nodebuilder.DiscoverOpened()
	if err != nil {
		return "", fmt.Errorf("token/node-store flag was not specified: %w", err)
	}

	return path, nil
}

func getToken(path string) (string, error) {
	if path == "" {
		return "", errors.New("root directory was not specified")
	}

	ks, err := newKeystore(path)
	if err != nil {
		return "", err
	}

	key, err := ks.Get(nodemod.SecretName)
	if err != nil {
		fmt.Printf("error getting the JWT secret: %v", err)
		return "", err
	}
	return buildJWTToken(key.Body, perms.AllPerms, 0)
}

type rpcClientKey struct{}

func ParseClientFromCtx(ctx context.Context) (*rpc.Client, error) {
	client, ok := ctx.Value(rpcClientKey{}).(*rpc.Client)
	if !ok {
		return nil, errors.New("rpc client was not set")
	}
	return client, nil
}
