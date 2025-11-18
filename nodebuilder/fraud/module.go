package fraud

import (
	"context"
	"errors"

	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	"go.uber.org/fx"

	"github.com/celestiaorg/go-fraud"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

var log = logging.Logger("module/fraud")

// stubFraudService is a no-op fraud service for when P2P is disabled.
type stubFraudService struct{}

func (s *stubFraudService) Get(_ context.Context, _ fraud.ProofType) ([]fraud.Proof[*header.ExtendedHeader], error) {
	// Return ErrNotFound so ServiceBreaker thinks there are no fraud proofs
	return nil, datastore.ErrNotFound
}

func (s *stubFraudService) Subscribe(_ fraud.ProofType) (fraud.Subscription[*header.ExtendedHeader], error) {
	return &stubFraudSubscription{}, nil
}

// stubFraudSubscription is a no-op subscription for when P2P is disabled.
type stubFraudSubscription struct{}

func (s *stubFraudSubscription) Proof(ctx context.Context) (fraud.Proof[*header.ExtendedHeader], error) {
	// Block forever - storage-only nodes don't receive fraud proofs via P2P
	<-ctx.Done()
	return nil, ctx.Err()
}

func (s *stubFraudSubscription) Cancel() {
	// No-op
}

func (s *stubFraudService) Broadcast(_ context.Context, _ fraud.Proof[*header.ExtendedHeader]) error {
	return errors.New("fraud service is disabled (P2P is disabled)")
}

func (s *stubFraudService) AddVerifier(_ fraud.ProofType, _ fraud.Verifier[*header.ExtendedHeader]) error {
	// No-op: storage-only nodes don't verify fraud proofs
	return nil
}

var (
	_ fraud.Service[*header.ExtendedHeader] = (*stubFraudService)(nil)
	_ Module                                = (*stubModule)(nil)
)

type stubModule struct {
	fraud.Service[*header.ExtendedHeader]
}

func (s *stubModule) Subscribe(_ context.Context, _ fraud.ProofType) (<-chan *Proof, error) {
	return nil, errors.New("fraud service is disabled (P2P is disabled)")
}

func (s *stubModule) Get(_ context.Context, _ fraud.ProofType) ([]Proof, error) {
	return nil, errors.New("fraud service is disabled (P2P is disabled)")
}

func newStubFraudService() (Module, fraud.Service[*header.ExtendedHeader], error) {
	stub := &stubFraudService{}
	return &stubModule{Service: stub}, stub, nil
}

func ConstructModule(tp node.Type, p2pCfg *modp2p.Config) fx.Option {
	p2pDisabled := p2pCfg != nil && p2pCfg.Disabled

	baseComponent := fx.Options(
		fx.Provide(Unmarshaler),
		fx.Provide(func(serv fraud.Service[*header.ExtendedHeader]) fraud.Getter[*header.ExtendedHeader] {
			return serv
		}),
	)

	// If P2P is disabled, provide stub fraud service
	if p2pDisabled && tp == node.Bridge {
		return fx.Module(
			"fraud",
			baseComponent,
			fx.Provide(newStubFraudService),
		)
	}

	switch tp {
	case node.Light:
		return fx.Module(
			"fraud",
			baseComponent,
			fx.Provide(newFraudServiceWithSync),
		)
	case node.Full, node.Bridge:
		return fx.Module(
			"fraud",
			baseComponent,
			fx.Provide(newFraudServiceWithoutSync),
		)
	default:
		panic("invalid node type")
	}
}
