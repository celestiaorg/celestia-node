package main

import (
	cmdnode "github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/nodebuilder/core"
	"github.com/celestiaorg/celestia-node/nodebuilder/gateway"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/rpc"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
	"github.com/spf13/cobra"
)

func persistentPreRunEnv(cmd *cobra.Command, nodeType node.Type, args []string) error {
	var (
		ctx = cmd.Context()
		err error
	)

	ctx = cmdnode.WithNodeType(ctx, nodeType)

	parsedNetwork, err := p2p.ParseNetwork(cmd)
	if err != nil {
		return err
	}
	ctx = cmdnode.WithNetwork(ctx, parsedNetwork)

	// loads existing config into the environment
	ctx, err = cmdnode.ParseNodeFlags(ctx, cmd, cmdnode.Network(ctx))
	if err != nil {
		return err
	}

	cfg := cmdnode.NodeConfig(ctx)

	err = p2p.ParseFlags(cmd, &cfg.P2P)
	if err != nil {
		return err
	}

	err = core.ParseFlags(cmd, &cfg.Core)
	if err != nil {
		return err
	}

	if nodeType != node.Bridge {
		err = header.ParseFlags(cmd, &cfg.Header)
		if err != nil {
			return err
		}
	}

	ctx, err = cmdnode.ParseMiscFlags(ctx, cmd)
	if err != nil {
		return err
	}

	rpc.ParseFlags(cmd, &cfg.RPC)
	gateway.ParseFlags(cmd, &cfg.Gateway)
	state.ParseFlags(cmd, &cfg.State)

	// set config
	ctx = cmdnode.WithNodeConfig(ctx, &cfg)
	cmd.SetContext(ctx)
	return nil
}
