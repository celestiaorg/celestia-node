package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/gateway"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/pruner"
	"github.com/celestiaorg/celestia-node/nodebuilder/rpc"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

func NewBridge(addFullFlags, addMinFlags func(*cobra.Command, []*pflag.FlagSet)) *cobra.Command {
	return createTopLevelCmd(node.Bridge, addFullFlags, addMinFlags)
}

func NewFull(addFullFlags, addMinFlags func(*cobra.Command, []*pflag.FlagSet)) *cobra.Command {
	return createTopLevelCmd(node.Full, addFullFlags, addMinFlags)
}

func NewLight(addFullFlags, addMinFlags func(*cobra.Command, []*pflag.FlagSet)) *cobra.Command {
	return createTopLevelCmd(node.Light, addFullFlags, addMinFlags)
}

func createTopLevelCmd(
	nodeType node.Type,
	addFullFlags,
	addMinFlags func(*cobra.Command, []*pflag.FlagSet),
) *cobra.Command {
	fullFlags := []*pflag.FlagSet{
		NodeFlags(),
		p2p.Flags(),
		MiscFlags(),
		core.Flags(),
		rpc.Flags(),
		gateway.Flags(),
		state.Flags(),
		pruner.Flags(),
	}
	if nodeType != node.Bridge {
		fullFlags = append(fullFlags, header.Flags())
	}

	minFlags := []*pflag.FlagSet{
		NodeStoreFlag(),
		p2p.NetworkFlag(),
	}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [subcommand]", strings.ToLower(nodeType.String())),
		Args:  cobra.NoArgs,
		Short: fmt.Sprintf("Manage your %s node", strings.ToLower(nodeType.String())),
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			ctx := WithNodeType(cmd.Context(), nodeType)
			cmd.SetContext(ctx)
		},
	}

	addFullFlags(cmd, fullFlags)
	addMinFlags(cmd, minFlags)

	return cmd
}
