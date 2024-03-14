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
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
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

	// try to detect a running node by checking for a lock file
	nodeTypes := []nodemod.Type{nodemod.Bridge, nodemod.Full, nodemod.Light}
	defaultNetwork := []p2p.Network{p2p.Mainnet, p2p.Mocha, p2p.Arabica, p2p.Private}
	for _, t := range nodeTypes {
		for _, n := range defaultNetwork {
			path, err := DefaultNodeStorePath(t.String(), n.String())
			if err != nil {
				return "", err
			}

			if nodebuilder.IsOpened(path) {
				return path, nil
			}
		}
	}

	return "", errors.New("cant get the access to the auth token: token/node-store flag was not specified")
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
