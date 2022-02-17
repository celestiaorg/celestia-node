package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/service/header"

	"github.com/celestiaorg/celestia-node/node"
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

		store, err := node.OpenStore(fmt.Sprintf("~/.celestia-%s", strings.ToLower(tp.String())), tp)
		if err != nil {
			return err
		}

		ds, err := store.Datastore()
		if err != nil {
			return err
		}

		hstore, err := header.NewStore(ds)
		if err != nil {
			return err
		}

		height, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("ivalid height: %w", err)
		}

		newHead, err := hstore.GetByHeight(cmd.Context(), uint64(height))
		if err != nil {
			return err
		}

		return hstore.Init(cmd.Context(), newHead)
	},
}
