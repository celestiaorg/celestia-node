package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/go-header/store"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder"
)

var headerCmd = &cobra.Command{
	Use:   "header [subcommand]",
	Short: "Collection of header module related utilities",
}

const (
	headFlag = "head"
	tailFlag = "tail"
)

const startFromFlag = "start-from"

var headerStoreReset = &cobra.Command{
	Use:          "store-reset <node_store_path> [--head <num>] [--tail <num>]",
	Short:        "Forcefully resets header store tail or head to be at the given height. Requires the node being stopped",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("not enough arguments")
		}
		ctx := cmd.Context()

		head, err := cmd.Flags().GetUint64("head")
		if err != nil {
			return err
		}
		tail, err := cmd.Flags().GetUint64("tail")
		if err != nil {
			return err
		}
		if head == 0 && tail == 0 {
			return errors.New("at least one of --head or --tail must be specified")
		}

		path := args[0]
		s, err := nodebuilder.OpenStore(path, nil)
		if err != nil {
			return err
		}
		defer func() {
			if err := s.Close(); err != nil {
				fmt.Printf("Error closing node store: %v\n", err)
			}
		}()

		ds, err := s.Datastore()
		if err != nil {
			return err
		}

		hstore, err := store.NewStore[*header.ExtendedHeader](ds)
		if err != nil {
			return fmt.Errorf("opening header store: %w", err)
		}
		if err = hstore.Start(ctx); err != nil {
			return err
		}
		defer func() {
			if err := hstore.Stop(ctx); err != nil {
				fmt.Printf("Error stopping header store: %v\n", err)
			}
		}()

		if head != 0 {
			err := store.UnsafeResetHead(ctx, hstore, head)
			if err != nil {
				return fmt.Errorf("resetting head: %w", err)
			}
			fmt.Println("Reset header store head to height", head)
		}
		if tail != 0 {
			err := store.UnsafeResetTail(ctx, hstore, tail)
			if err != nil {
				return fmt.Errorf("resetting tail: %w", err)
			}
			fmt.Println("Reset header store tail to height", tail)
		}

		return nil
	},
}

var headerStoreRecover = &cobra.Command{
	Use: "store-recover <node_store_path> [--start-from <num>]",
	Short: `Recovers header store tail by forward iterating over the store until some header is found to be the new tail.
	Requires the node being stopped`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("not enough arguments")
		}
		ctx := cmd.Context()

		path := args[0]
		s, err := nodebuilder.OpenStore(path, nil)
		if err != nil {
			return err
		}
		defer func() {
			if err := s.Close(); err != nil {
				fmt.Printf("Error closing node store: %v\n", err)
			}
		}()
		ds, err := s.Datastore()
		if err != nil {
			return err
		}

		hstore, err := store.NewStore[*header.ExtendedHeader](ds)
		if err != nil {
			return fmt.Errorf("opening header store: %w", err)
		}
		if err = hstore.Start(ctx); err != nil {
			return err
		}
		defer func() {
			if err := hstore.Stop(ctx); err != nil {
				fmt.Printf("Error stopping header store: %v\n", err)
			}
		}()

		startFrom, err := cmd.Flags().GetUint64(startFromFlag)
		if err != nil {
			return err
		}

		hdr, err := store.FindHeader(ctx, hstore, startFrom)
		if err != nil {
			return fmt.Errorf("finding header at height %d: %w", startFrom, err)
		}
		fmt.Println("Found header at height", hdr.Height())

		err = store.UnsafeResetTail(ctx, hstore, hdr.Height())
		if err != nil {
			return fmt.Errorf("resetting header store tail to %d: %w", hdr.Height(), err)
		}
		fmt.Println("Reset header store tail to height", hdr.Height())

		return nil
	},
}

func init() {
	headerCmd.AddCommand(headerStoreReset)
	headerCmd.AddCommand(headerStoreRecover)

	headerStoreReset.Flags().Uint64(headFlag, 0, "desired head height")
	headerStoreReset.Flags().Uint64(tailFlag, 0, "desired tail height")
	headerStoreRecover.Flags().Uint64(startFromFlag, 1, "starts iterating from the given block height")
}
