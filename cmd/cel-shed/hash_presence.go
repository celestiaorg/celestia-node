package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/store"
)

const (
	nodeStorePathFlag = "node.store"
)

func init() {
	hashCmd.PersistentFlags().String("node.store", "", "Path to the node store")

	hashCmd.AddCommand(hasHashCmd, getByHashCmd)
}

var hashCmd = &cobra.Command{
	Use:   "hash [subcommand] [flags]",
	Short: "Commands to interact with EDS hashes",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		path, err := cmd.Flags().GetString(nodeStorePathFlag)
		if err != nil {
			return err
		}

		rootHash, err := parseHash(args[0])
		if err != nil {
			return err
		}
		st, err := store.NewStore(store.DefaultParameters(), path)
		if err != nil {
			return fmt.Errorf("creating a store: %w", err)
		}

		ctx := cmd.Context()

		ctx = context.WithValue(ctx, hashKey{}, rootHash)
		ctx = context.WithValue(ctx, edsStoreKey{}, st)
		cmd.SetContext(ctx)
		return nil
	},
}

var hasHashCmd = &cobra.Command{
	Use:     "has [root hash] [flags]",
	Aliases: []string{"exists"},
	Short:   "Command to check whether a hash is present in the eds store",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, _ []string) error {
		rootHash, store, err := parseDataFromContext(cmd.Context())
		if err != nil {
			return err
		}

		exists, err := store.HasByHash(context.Background(), rootHash)
		if err != nil {
			return fmt.Errorf("checking for root hash: %w", err)
		}

		if exists {
			fmt.Printf("Root hash %s exists.\n", rootHash)
		} else {
			fmt.Printf("Root hash %s does not exist.\n", rootHash)
		}
		return nil
	},
}

var getByHashCmd = &cobra.Command{
	Use:   "get [root hash] [flag]",
	Short: "Command to get the eds from the eds store by its hash",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, _ []string) error {
		rootHash, store, err := parseDataFromContext(cmd.Context())
		if err != nil {
			return err
		}

		eds, err := store.GetByHash(context.Background(), rootHash)
		if err != nil {
			return err
		}

		fmt.Printf("Retrieved EDS for root hash: %s\n\n", rootHash)

		reader, err := eds.Reader()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			return err
		}
		output, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}

		fmt.Println(string(output))
		return nil
	},
}

func parseHash(str string) (root share.DataHash, err error) {
	hexData, err := hex.DecodeString(str)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex string: %w", err)
	}
	return hexData, nil
}

type (
	hashKey     struct{}
	edsStoreKey struct{}
)

func parseDataFromContext(ctx context.Context) (share.DataHash, *store.Store, error) {
	hash, ok := ctx.Value(hashKey{}).(share.DataHash)
	if !ok {
		return nil, nil, fmt.Errorf("root hash not found in context")
	}

	st, ok := ctx.Value(edsStoreKey{}).(*store.Store)
	if !ok {
		return nil, nil, fmt.Errorf("eds store not found in context")
	}
	return hash, st, nil
}
