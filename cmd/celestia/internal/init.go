package internal

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/api/rpc/client"
)

const authEnvKey = "CELESTIA_NODE_AUTH_TOKEN" //nolint:gosec

var (
	RPCClient  *client.Client
	RequestURL string
)

// InitClient creates the rpc client under the given at the *RequestURL* address
func InitClient(cmd *cobra.Command, _ []string) error {
	var err error
	RPCClient, err = client.NewClient(cmd.Context(), RequestURL, os.Getenv(authEnvKey))
	if err != nil {
		return err
	}
	return nil
}

// CloseClient closes the connection with the rpc client.
func CloseClient(_ *cobra.Command, _ []string) {
	if RPCClient != nil {
		RPCClient.Close()
	}
}
