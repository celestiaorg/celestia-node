package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	rpc "github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/api/rpc/perms"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	nodemod "github.com/celestiaorg/celestia-node/nodebuilder/node"
)

const (
	// defaultRPCAddress is a default address to dial to
	defaultRPCAddress = "http://localhost:26658"
)

var (
	requestURL    string
	authTokenFlag string
)

func RPCFlags() *flag.FlagSet {
	fset := &flag.FlagSet{}

	fset.StringVar(
		&requestURL,
		"url",
		defaultRPCAddress,
		"Request URL",
	)

	fset.StringVar(
		&authTokenFlag,
		"token",
		"",
		"Authorization token",
	)

	storeFlag := NodeFlags().Lookup(nodeStoreFlag)
	fset.AddFlag(storeFlag)
	return fset
}

func InitClient(cmd *cobra.Command, _ []string) error {
	if authTokenFlag == "" {
		storePath, err := getStorePath(cmd)
		if err != nil {
			return err
		}

		token, err := getToken(storePath)
		if err != nil {
			return fmt.Errorf("cant get the access to the auth token: %v", err)
		}
		authTokenFlag = token
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

	errMsg := "cant get the access to the auth token: token/node-store flag was not specified"

	// try to detect a running node
	path, err := nodebuilder.DiscoverOpened()
	if err != nil {
		return "", fmt.Errorf("%s: %w", errMsg, err)
	}

	if path != "" {
		return path, nil
	}

	return "", errors.New(errMsg)
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
	return buildJWTToken(key.Body, perms.AllPerms)
}

type rpcClientKey struct{}

func ParseClientFromCtx(ctx context.Context) (*rpc.Client, error) {
	client, ok := ctx.Value(rpcClientKey{}).(*rpc.Client)
	if !ok {
		return nil, errors.New("rpc client was not set")
	}
	return client, nil
}
