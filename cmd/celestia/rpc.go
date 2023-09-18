package main

import (
	"github.com/celestiaorg/celestia-node/api/rpc/client"

	blob "github.com/celestiaorg/celestia-node/nodebuilder/blob/cmds"
	das "github.com/celestiaorg/celestia-node/nodebuilder/das/cmds"
	header "github.com/celestiaorg/celestia-node/nodebuilder/header/cmds"
	node "github.com/celestiaorg/celestia-node/nodebuilder/node/cmds"
	p2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p/cmds"
	share "github.com/celestiaorg/celestia-node/nodebuilder/share/cmds"
	state "github.com/celestiaorg/celestia-node/nodebuilder/state/cmds"
)

const authEnvKey = "CELESTIA_NODE_AUTH_TOKEN" //nolint:gosec

func init() {
	blob.BlobCmd.PersistentFlags().StringVar(initUrlFlag())
	das.DASCmd.PersistentFlags().StringVar(initUrlFlag())
	header.HeaderCmd.PersistentFlags().StringVar(initUrlFlag())
	p2p.P2PCmd.PersistentFlags().StringVar(initUrlFlag())
	share.ShareCmd.PersistentFlags().StringVar(initUrlFlag())
	state.StateCmd.PersistentFlags().StringVar(initUrlFlag())
	node.NodeCmd.PersistentFlags().StringVar(initUrlFlag())

	rootCmd.AddCommand(
		blob.BlobCmd,
		das.DASCmd,
		header.HeaderCmd,
		p2p.P2PCmd,
		share.ShareCmd,
		state.StateCmd,
		node.NodeCmd,
	)
}

func initUrlFlag() (*string, string, string, string) {
	return &client.RequestURL, "url", client.DefaultRPCAddress, "Request URL"
}
