package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/ipfs/boxo/blockstore"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	dsq "github.com/ipfs/go-datastore/query"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/celestiaorg/celestia-node/nodebuilder"
)

func init() {
	datastoreCmd.AddCommand(eraseCmd, eraseSamplesCmd)
}

var datastoreCmd = &cobra.Command{
	Use:   "datastore [subcommand]",
	Short: "Collection of datastore related utilities",
}

var eraseCmd = &cobra.Command{
	Use:   "erase <ds_key>",
	Short: "Erase datastore namespace",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := nodebuilder.DiscoverStopped()
		if err != nil {
			return fmt.Errorf("discovering stopped node: %w", err)
		}
		fmt.Printf("Discovered stopped node at %s\n", path)

		nodestore, err := nodebuilder.OpenStore(path, nil)
		if err != nil {
			return err
		}
		defer func() {
			err = errors.Join(err, nodestore.Close())
		}()

		store, err := nodestore.Datastore()
		if err != nil {
			return fmt.Errorf("getting datastore: %w", err)
		}

		key := ds.NewKey(args[0])
		err = eraseDatastoreNamespace(cmd.Context(), store, key)
		if err != nil {
			return fmt.Errorf("erasing datastore namespace: %w", err)
		}

		fmt.Printf("Erased %s\n", key)
		return nil
	},
	Args: cobra.ExactArgs(1),
}

var sampleDataKeys = []ds.Key{
	ds.NewKey("sampling_result"),
	ds.NewKey("das"),
	ds.NewKey("pruner"),
	blockstore.BlockPrefix,
}

var eraseSamplesCmd = &cobra.Command{
	Use:   "erase-samples [subcommand]",
	Short: "Erase samples data and state. Useful to resample, avoiding resyncing headers",
	RunE: func(cmd *cobra.Command, _ []string) error {
		path, err := nodebuilder.DiscoverStopped()
		if err != nil {
			return fmt.Errorf("discovering stopped node: %w", err)
		}
		fmt.Printf("Discovered stopped node at %s\n", path)

		nodestore, err := nodebuilder.OpenStore(path, nil)
		if err != nil {
			return err
		}
		defer func() {
			err = errors.Join(err, nodestore.Close())
		}()

		ds, err := nodestore.Datastore()
		if err != nil {
			return fmt.Errorf("getting datastore: %w", err)
		}

		group, ctx := errgroup.WithContext(cmd.Context())
		for _, key := range sampleDataKeys {
			group.Go(func() error {
				err := eraseDatastoreNamespace(ctx, ds, key)
				if err != nil {
					return fmt.Errorf("erasing datastore namespace: %w", err)
				}
				fmt.Printf("Erased %s\n", key)
				return nil
			})
		}

		return group.Wait()
	},
}

func eraseDatastoreNamespace(ctx context.Context, store ds.Datastore, key ds.Key) error {
	store = namespace.Wrap(store, key)

	q := dsq.Query{KeysOnly: true}
	res, err := store.Query(ctx, q)
	if err != nil {
		return fmt.Errorf("querying datastore: %w", err)
	}

	for {
		e, ok := res.NextSync()
		if !ok {
			break
		}
		if e.Error != nil {
			return fmt.Errorf("getting next key: %w", e.Error)
		}

		key := ds.RawKey(e.Key)
		err := store.Delete(ctx, key)
		if err != nil {
			return fmt.Errorf("deleting key: %w", err)
		}
	}

	return nil
}
