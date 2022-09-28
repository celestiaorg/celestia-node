package fraud

import (
	"context"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p-core/host"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"go.uber.org/fx"

	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/header"
)

// NewModule constructs a fraud proof service with the syncer disabled.
func NewModule(
	lc fx.Lifecycle,
	sub *pubsub.PubSub,
	host host.Host,
	hstore header.Store,
	ds datastore.Batching,
) (Module, error) {
	return newFraudService(lc, sub, host, hstore, ds, false)
}

// ModuleWithSyncer constructs fraud proof service with enabled syncer.
func ModuleWithSyncer(
	lc fx.Lifecycle,
	sub *pubsub.PubSub,
	host host.Host,
	hstore header.Store,
	ds datastore.Batching,
) (Module, error) {
	return newFraudService(lc, sub, host, hstore, ds, true)
}

func newFraudService(
	lc fx.Lifecycle,
	sub *pubsub.PubSub,
	host host.Host,
	hstore header.Store,
	ds datastore.Batching,
	isEnabled bool) (Module, error) {
	pservice := fraud.NewProofService(sub, host, hstore.GetByHeight, ds, isEnabled)
	lc.Append(fx.Hook{
		OnStart: pservice.Start,
		OnStop:  pservice.Stop,
	})
	return pservice, nil
}

// Lifecycle controls the lifecycle of service depending on fraud proofs.
// It starts the service only if no fraud-proof exists and stops the service automatically
// if a proof arrives after the service was started.
func Lifecycle(
	startCtx, lifecycleCtx context.Context,
	p fraud.ProofType,
	fraudModule Module,
	start, stop func(context.Context) error,
) error {
	proofs, err := fraudModule.Get(startCtx, p)
	switch err {
	default:
		return err
	case nil:
		return &fraud.ErrFraudExists{Proof: proofs}
	case datastore.ErrNotFound:
	}
	err = start(startCtx)
	if err != nil {
		return err
	}
	// handle incoming Fraud Proofs
	go fraud.OnProof(lifecycleCtx, fraudModule, p, func(fraud.Proof) {
		if err := stop(lifecycleCtx); err != nil {
			log.Error(err)
		}
	})
	return nil
}
