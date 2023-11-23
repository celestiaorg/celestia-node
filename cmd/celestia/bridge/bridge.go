package bridge

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

func NewCommand(options ...func(*cobra.Command, []*pflag.FlagSet)) *cobra.Command {
	flags := []*pflag.FlagSet{
		cmdnode.NodeFlags(),
		p2p.Flags(),
		cmdnode.MiscFlags(),
		core.Flags(),
		rpc.Flags(),
		gateway.Flags(),
		state.Flags(),
	}
	cmd := &cobra.Command{
		Use:   "bridge [subcommand]",
		Args:  cobra.NoArgs,
		Short: "Manage your Bridge node",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return cmdnode.PersistentPreRunEnv(cmd, node.Bridge, args)
		},
	}
	for _, option := range options {
		option(cmd, flags)
	}
	return cmd
}
