package main

import (
	"github.com/celestiaorg/celestia-node/cmd/celestia/internal/admin"
	"github.com/celestiaorg/celestia-node/cmd/celestia/internal/rpc"
)

func init() {
	rootCmd.AddCommand(admin.NodeCmd)

	rootCmd.AddCommand(
		rpc.BlobCmd,
		rpc.DASCmd,
		rpc.HeaderCmd,
		rpc.P2PCmd,
		rpc.ShareCmd,
		rpc.StateCmd,
	)
}
