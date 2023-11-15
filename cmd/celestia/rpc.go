package main

import (
	"github.com/celestiaorg/celestia-node/cmd"
	blob "github.com/celestiaorg/celestia-node/nodebuilder/blob/cmd"
	das "github.com/celestiaorg/celestia-node/nodebuilder/das/cmd"
	header "github.com/celestiaorg/celestia-node/nodebuilder/header/cmd"
	node "github.com/celestiaorg/celestia-node/nodebuilder/node/cmd"
	p2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p/cmd"
	share "github.com/celestiaorg/celestia-node/nodebuilder/share/cmd"
	state "github.com/celestiaorg/celestia-node/nodebuilder/state/cmd"
)

func init() {
	blob.Cmd.PersistentFlags().AddFlagSet(cmd.RPCFlags())
	das.Cmd.PersistentFlags().AddFlagSet(cmd.RPCFlags())
	header.Cmd.PersistentFlags().AddFlagSet(cmd.RPCFlags())
	p2p.Cmd.PersistentFlags().AddFlagSet(cmd.RPCFlags())
	share.Cmd.PersistentFlags().AddFlagSet(cmd.RPCFlags())
	state.Cmd.PersistentFlags().AddFlagSet(cmd.RPCFlags())
	node.Cmd.PersistentFlags().AddFlagSet(cmd.RPCFlags())

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
