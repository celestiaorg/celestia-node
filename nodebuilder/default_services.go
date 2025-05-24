package nodebuilder

import (
	"github.com/celestiaorg/celestia-node/nodebuilder/blob"
	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/modname"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

// PackageToAPI maps a package to its API struct. Currently only used for
// method discovery for openrpc spec generation
var PackageToAPI = map[string]any{
	modname.Fraud:  &fraud.API{},
	modname.State:  &state.API{},
	modname.Share:  &share.API{},
	modname.Header: &header.API{},
	modname.DAS:    &das.API{},
	modname.P2P:    &p2p.API{},
	modname.Blob:   &blob.API{},
	modname.Node:   &node.API{},
}
