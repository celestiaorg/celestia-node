package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"

	"github.com/spf13/cobra"
)

var path string

func init() {
	rootCmd.PersistentFlags().StringVar(&path, "node.store", "", "Path to the node store")
}

var hashCmd = &cobra.Command{
	Use:   "hash [command] [root hash]",
	Short: "Commands to interact with EDS hashes",
	Args:  cobra.ExactArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		command := args[0]

		hexData, err := hex.DecodeString(args[1])
		if err != nil {
			return fmt.Errorf("failed to decode hex string: %w", err)
		}
		rootHash := share.DataHash(hexData)

		nodestore, err := nodebuilder.OpenStore(path, nil)
		if err != nil {
			return err
		}
		defer func() {
			err = errors.Join(err, nodestore.Close())
		}()

		datastore, err := nodestore.Datastore()
		if err != nil {
			return err
		}

		store, err := eds.NewStore(eds.DefaultParameters(), path, datastore)
		if err != nil {
			return err
		}
		err = store.Start(context.Background())
		if err != nil {
			fmt.Println("Error starting store:", err)
			return err
		}

		switch command {
		case "has", "exists":
			exists, err := store.Has(context.Background(), rootHash)
			if err != nil {
				fmt.Println("Error checking for root hash:", err)
				return err
			}

			if exists {
				fmt.Printf("Root hash %s exists.\n", rootHash)
			} else {
				fmt.Printf("Root hash %s does not exist.\n", rootHash)
			}
		case "get":
			eds, err := store.Get(context.Background(), rootHash)
			if err != nil {
				return err
			}

			fmt.Printf("Retrieved EDS for root hash %s\n", rootHash)

			output, err := json.MarshalIndent(eds, "", "  ")
			if err != nil {
				return err
			}

			fmt.Println(string(output))
		default:
			return fmt.Errorf("unknown command: %s", command)
		}

		return nil
	},
}
