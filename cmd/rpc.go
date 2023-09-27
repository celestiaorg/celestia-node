package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	rpc "github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/api/rpc/perms"
	nodemod "github.com/celestiaorg/celestia-node/nodebuilder/node"
)

const (
	// defaultRPCAddress is a default address to dial to
	defaultRPCAddress = "http://localhost:26658"
	authEnvKey        = "CELESTIA_NODE_AUTH_TOKEN" //nolint:gosec
)

var (
	requestURL    string
	authTokenFlag string
	storePath     string
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
		"Authorization token (if not provided, the "+authEnvKey+" environment variable will be used)",
	)

	storeFlag := NodeFlags().Lookup(nodeStoreFlag)

	fset.StringVar(
		&storePath,
		nodeStoreFlag,
		"",
		storeFlag.Usage,
	)
	return fset
}

func InitClient(cmd *cobra.Command, _ []string) error {
	if authTokenFlag == "" {
		authTokenFlag = os.Getenv(authEnvKey)
	}

	if authTokenFlag == "" {
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
