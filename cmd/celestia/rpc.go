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
	blob.Cmd.PersistentFlags().StringVar(cmd.InitURLFlag())
	das.Cmd.PersistentFlags().StringVar(cmd.InitURLFlag())
	header.Cmd.PersistentFlags().StringVar(cmd.InitURLFlag())
	p2p.Cmd.PersistentFlags().StringVar(cmd.InitURLFlag())
	share.Cmd.PersistentFlags().StringVar(cmd.InitURLFlag())
	state.Cmd.PersistentFlags().StringVar(cmd.InitURLFlag())
	node.Cmd.PersistentFlags().StringVar(cmd.InitURLFlag())

	blob.Cmd.PersistentFlags().StringVar(cmd.InitAuthTokenFlag())
	das.Cmd.PersistentFlags().StringVar(cmd.InitAuthTokenFlag())
	header.Cmd.PersistentFlags().StringVar(cmd.InitAuthTokenFlag())
	p2p.Cmd.PersistentFlags().StringVar(cmd.InitAuthTokenFlag())
	share.Cmd.PersistentFlags().StringVar(cmd.InitAuthTokenFlag())
	state.Cmd.PersistentFlags().StringVar(cmd.InitAuthTokenFlag())
	node.Cmd.PersistentFlags().StringVar(cmd.InitAuthTokenFlag())

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
