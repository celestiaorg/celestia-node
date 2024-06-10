package main

import (
	"context"
	"fmt"

	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/spf13/cobra"
)

var path string

func init() {
	rootCmd.PersistentFlags().StringVar(&path, "path", "", "Path to the node store")
}

var hashCmd = &cobra.Command{
	Use:   "hash [command] [root hash]",
	Short: "Commands to interact with EDS hashes",
	Args:  cobra.ExactArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		command := args[0]
		rootHash := share.DataHash(args[1])

		nodestore, err := nodebuilder.OpenStore(path, nil)
		if err != nil {
			return err
		}

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

			fmt.Printf("Retrieved EDS for root hash %s: %v\n", rootHash, eds)
		default:
			return fmt.Errorf("unknown command: %s", command)
		}

		return nil
	},
}
