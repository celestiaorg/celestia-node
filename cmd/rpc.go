package cmd

import (
	"context"
	"errors"
	"os"

	"github.com/spf13/cobra"

	rpc "github.com/celestiaorg/celestia-node/api/rpc/client"
)

const (
	// defaultRPCAddress is a default address to dial to
	defaultRPCAddress = "http://localhost:26658"
	authEnvKey        = "CELESTIA_NODE_AUTH_TOKEN" //nolint:gosec
)

var (
	requestURL    string
	authTokenFlag string
)

func InitURLFlag() (*string, string, string, string) {
	return &requestURL, "url", defaultRPCAddress, "Request URL"
}

func InitAuthTokenFlag() (*string, string, string, string) {
	return &authTokenFlag,
		"token",
		"",
		"Authorization token (if not provided, the " + authEnvKey + " environment variable will be used)"
}

func InitClient(cmd *cobra.Command, _ []string) error {
	if authTokenFlag == "" {
		authTokenFlag = os.Getenv(authEnvKey)
	}

	client, err := rpc.NewClient(cmd.Context(), requestURL, authTokenFlag)
	if err != nil {
		return err
	}

	ctx := context.WithValue(cmd.Context(), rpcClientKey{}, client)
	cmd.SetContext(ctx)
	return nil
}

type rpcClientKey struct{}

func ParseClientFromCtx(ctx context.Context) (*rpc.Client, error) {
	client, ok := ctx.Value(rpcClientKey{}).(*rpc.Client)
	if !ok {
		return nil, errors.New("rpc client was not set")
	}
	return client, nil
}
