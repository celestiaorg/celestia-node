package blob

import (
	"github.com/ipfs/go-blockservice"
	"go.uber.org/fx"

	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/libs/fxutil"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/state"
)

func ConstructModule(tp node.Type) fx.Option {
	options := make([]fx.Option, 0)
	switch tp {
	case node.Full, node.Bridge:
		options = append(options,
			fxutil.ProvideAs(func(store libhead.Store[*header.ExtendedHeader]) libhead.Getter[*header.ExtendedHeader] {
				return store
			}))
	case node.Light:
		options = append(options,
			fxutil.ProvideAs(func(exchange libhead.Exchange[*header.ExtendedHeader]) libhead.Getter[*header.ExtendedHeader] {
				return exchange
			}),
		)
	default:
		panic("invalid node type")
	}

	return fx.Module("blob",
		fx.Options(options...),
		fx.Provide(fx.Annotate(func(
			state *state.CoreAccessor,
			bGetter blockservice.BlockService,
			hGetter libhead.Getter[*header.ExtendedHeader],
			headGetter *sync.Syncer[*header.ExtendedHeader]) Module {
			return blob.NewService(state, bGetter, hGetter, headGetter)
		})))
}
