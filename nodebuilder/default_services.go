package nodebuilder

import (
	"github.com/celestiaorg/celestia-node/nodebuilder/das"
	"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/share"
	"github.com/celestiaorg/celestia-node/nodebuilder/state"
)

// PackageToAPI maps a package to its API struct. Currently only used for
// method discovery for openrpc spec generation
var PackageToAPI = map[string]interface{}{
	"fraud":  &fraud.API{},
	"state":  &state.API{},
	"share":  &share.API{},
	"header": &header.API{},
	"daser":  &das.API{},
	"p2p":    &p2p.API{},
	"node":   &node.API{},
}
