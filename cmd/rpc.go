package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cristalhq/jwt"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	rpc "github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/api/rpc/perms"
	"github.com/celestiaorg/celestia-node/libs/authtoken"
	"github.com/celestiaorg/celestia-node/libs/keystore"
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
	var (
		err   error
		token string
	)

	if authTokenFlag == "" {
		token, err = getToken(storePath)
	}

	if err != nil {
		token = os.Getenv(authEnvKey)
	}

	client, err := rpc.NewClient(cmd.Context(), requestURL, token)
	if err != nil {
		return err
	}

	ctx := context.WithValue(cmd.Context(), rpcClientKey{}, client)
	cmd.SetContext(ctx)
	return nil
}

func getToken(path string) (string, error) {
	if path == "" {
		return "", errors.New("empty path")
	}

	expanded, err := homedir.Expand(filepath.Clean(path))
	if err != nil {
		fmt.Printf("error expanding the root dir path:%v", err)
		return "", err
	}

	ks, err := keystore.NewFSKeystore(filepath.Join(expanded, "keys"), nil)
	if err != nil {
		fmt.Printf("error creating the keystore:%v", err)
		return "", err
	}

	var key keystore.PrivKey
	key, err = ks.Get(nodemod.SecretName)
	if err != nil {
		fmt.Printf("error getting the private key:%v", err)
		return "", err
	}

	signer, err := jwt.NewHS256(key.Body)
	if err != nil {
		fmt.Printf("error creating a signer:%v", err)
		return "", err
	}

	token, err := authtoken.NewSignedJWT(signer, perms.AllPerms)
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	authTokenFlag = token
	return token, nil
}

type rpcClientKey struct{}

func ParseClientFromCtx(ctx context.Context) (*rpc.Client, error) {
	client, ok := ctx.Value(rpcClientKey{}).(*rpc.Client)
	if !ok {
		return nil, errors.New("rpc client was not set")
	}
	return client, nil
}
