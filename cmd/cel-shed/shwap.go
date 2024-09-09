package main

import (
	"fmt"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/bitswap"
	"github.com/ipfs/go-cid"
	"github.com/spf13/cobra"
	"reflect"
)

func init() {
	shwapCmd.AddCommand(shwapCIDType)
}

var shwapCmd = &cobra.Command{
	Use:   "shwap [subcommand]",
	Short: "Collection of shwap related utilities",
}

var shwapCIDType = &cobra.Command{
	Use:   "cid-type",
	Short: "Decodes Bitswap CID composed over Shwap CID",
	RunE: func(_ *cobra.Command, args []string) error {
		cid, err := cid.Decode(args[0])
		if err != nil {
			return fmt.Errorf("decoding cid: %w", err)
		}

		blk, err := bitswap.EmptyBlock(cid)
		if err != nil {
			return fmt.Errorf("building block: %w", err)
		}

		fmt.Printf("%s: %+v\n", reflect.TypeOf(blk), blk)
		return nil
	},
	Args: cobra.ExactArgs(1),
}
