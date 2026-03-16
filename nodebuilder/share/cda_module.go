package share

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/crypto"
)

// CDAModule provides CDA functionality (early scaffold).
//
// Note: This is an incremental wiring that uses:
// - Noop KZG verifier, and
// - static commitments provider
// to unblock networking/storage integration. It will be replaced once the
// ExtendedHeader carries real KZG commitments and the gnark-crypto backed
// verifier/prover are integrated.
type CDAModule struct {
	Node *share.CDANode
}

func cdaComponents(tp node.Type, cfg *Config) fx.Option {
	_ = tp
	if !cfg.CDAEnabled {
		return fx.Options()
	}

	return fx.Options(
		fx.Provide(newCDANode),
		fx.Provide(func(n *share.CDANode) *CDAModule { return &CDAModule{Node: n} }),
		fx.Invoke(func(lc fx.Lifecycle, n *share.CDANode) {
			lc.Append(fx.Hook{
				OnStart: n.Start,
				OnStop:  n.Stop,
			})
		}),
	)
}

func newCDANode(
	h host.Host,
	ps *pubsub.PubSub,
	hs header.Module,
	cfg Config,
) (*share.CDANode, error) {
	// Compute grid dimensions consistent with RDA.
	expectedNodes := cfg.RDAExpectedNodeCount
	if expectedNodes <= 0 {
		expectedNodes = 16384
	}
	gridDims := cfg.RDAGridDimensions
	if expectedNodes > 0 {
		gridDims = share.CalculateOptimalGridSize(uint32(expectedNodes))
	}

	gridMgr := share.NewRDAGridManager(gridDims)
	gridMgr.RegisterPeer(h.ID())

	k := uint16(cfg.CDAK)
	if k == 0 {
		k = 16
	}

	// buffer defaults to 4 (k+4 cap).
	buffer := uint16(4)
	if cfg.CDABuffer > 0 {
		buffer = uint16(cfg.CDABuffer)
	}

	serviceCfg := share.CDAServiceConfig{
		K:      k,
		Buffer: buffer,
	}

	nodeCfg := share.CDANodeConfig{Service: serviceCfg}

	verifier := crypto.NewBLS12381HomomorphicKZG()
	commitSrc := share.NewHeaderColumnCommitmentProvider(hs)

	return share.NewCDANode(
		h,
		ps,
		gridMgr,
		verifier,
		commitSrc,
		nodeCfg,
	), nil
}

