package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/header/store"
	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func init() {
	headerCmd.AddCommand(headerStoreInit)
}

var headerCmd = &cobra.Command{
	Use:   "header [subcommand]",
	Short: "Collection of header module related utilities",
}

var headerStoreInit = &cobra.Command{
	Use: "store-init [node-type] [height]",
	Short: `Forcefully initialize header store head to be of the given height. Requires the node being stopped. 
Custom store path is not supported yet.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 2 {
			return fmt.Errorf("not enough arguments")
		}

		tp := node.ParseType(args[0])
		if !tp.IsValid() {
			return fmt.Errorf("invalid node-type")
		}

		height, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid height: %w", err)
		}

		s, err := nodebuilder.OpenStore(fmt.Sprintf("~/.celestia-%s", strings.ToLower(tp.String())))
		if err != nil {
			return err
		}

		ds, err := s.Datastore()
		if err != nil {
			return err
		}

		hstore, err := store.NewStore(ds)
		if err != nil {
			return err
		}

		newHead, err := hstore.GetByHeight(cmd.Context(), uint64(height))
		if err != nil {
			return err
		}

		return hstore.Init(cmd.Context(), newHead)
	},
}
