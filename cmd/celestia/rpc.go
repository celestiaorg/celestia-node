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

func init() {
	blob.Cmd.PersistentFlags().StringVar(initURLFlag())
	das.Cmd.PersistentFlags().StringVar(initURLFlag())
	header.Cmd.PersistentFlags().StringVar(initURLFlag())
	p2p.Cmd.PersistentFlags().StringVar(initURLFlag())
	share.Cmd.PersistentFlags().StringVar(initURLFlag())
	state.Cmd.PersistentFlags().StringVar(initURLFlag())
	node.Cmd.PersistentFlags().StringVar(initURLFlag())

	rootCmd.AddCommand(
		blob.Cmd,
		das.Cmd,
		header.Cmd,
		p2p.Cmd,
		share.Cmd,
		state.Cmd,
		node.Cmd,
	)
}

func initURLFlag() (*string, string, string, string) {
	return &client.RequestURL, "url", client.DefaultRPCAddress, "Request URL"
}
