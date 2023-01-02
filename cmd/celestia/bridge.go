package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	cmdnode "github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/gateway"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/rpc"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

// NOTE: We should always ensure that the added Flags below are parsed somewhere, like in the
// PersistentPreRun func on parent command.

func init() {
	flags := []*pflag.FlagSet{
		cmdnode.NodeFlags(),
		p2p.Flags(),
		core.Flags(),
		cmdnode.MiscFlags(),
		rpc.Flags(),
		gateway.Flags(),
		state.Flags(),
	}

	bridgeCmd.AddCommand(
		cmdnode.Init(flags...),
		cmdnode.Start(flags...),
		cmdnode.AuthCmd(flags...),
	)
}

var bridgeCmd = &cobra.Command{
	Use:   "bridge [subcommand]",
	Args:  cobra.NoArgs,
	Short: "Manage your Bridge node",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return persistentPreRunEnv(cmd, node.Bridge, args)
	},
}
