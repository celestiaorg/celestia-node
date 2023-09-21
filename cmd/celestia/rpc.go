package main

import (
	"github.com/celestiaorg/celestia-node/cmd/celestia/util"
	blob "github.com/celestiaorg/celestia-node/nodebuilder/blob/cmd"
	das "github.com/celestiaorg/celestia-node/nodebuilder/das/cmd"
	header "github.com/celestiaorg/celestia-node/nodebuilder/header/cmd"
	node "github.com/celestiaorg/celestia-node/nodebuilder/node/cmd"
	p2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p/cmd"
	share "github.com/celestiaorg/celestia-node/nodebuilder/share/cmd"
	state "github.com/celestiaorg/celestia-node/nodebuilder/state/cmd"
)

func init() {
	blob.Cmd.PersistentFlags().StringVar(util.InitURLFlag())
	das.Cmd.PersistentFlags().StringVar(util.InitURLFlag())
	header.Cmd.PersistentFlags().StringVar(util.InitURLFlag())
	p2p.Cmd.PersistentFlags().StringVar(util.InitURLFlag())
	share.Cmd.PersistentFlags().StringVar(util.InitURLFlag())
	state.Cmd.PersistentFlags().StringVar(util.InitURLFlag())
	node.Cmd.PersistentFlags().StringVar(util.InitURLFlag())

	blob.Cmd.PersistentFlags().StringVar(util.InitAuthTokenFlag())
	das.Cmd.PersistentFlags().StringVar(util.InitAuthTokenFlag())
	header.Cmd.PersistentFlags().StringVar(util.InitAuthTokenFlag())
	p2p.Cmd.PersistentFlags().StringVar(util.InitAuthTokenFlag())
	share.Cmd.PersistentFlags().StringVar(util.InitAuthTokenFlag())
	state.Cmd.PersistentFlags().StringVar(util.InitAuthTokenFlag())
	node.Cmd.PersistentFlags().StringVar(util.InitAuthTokenFlag())

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
