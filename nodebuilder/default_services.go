package nodebuilder

import (
	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/share/availability/light"
	"github.com/celestiaorg/celestia-node/state"
)

// PackageToDefaultImpl maps a package to its default implementation. Currently only used for method discovery for
// openrpc spec generation
var PackageToDefaultImpl = map[string]interface{}{
	"fraud":  &fraud.ProofService{},
	"state":  &state.CoreAccessor{},
	"share":  &light.ShareAvailability{},
	"header": &header.Service{},
	"daser":  &das.DASer{},
}
