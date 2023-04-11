package fraud

import (
	"context"
	"time"

	"github.com/ipfs/go-datastore"

	"github.com/celestiaorg/celestia-node/libs/fraud"
)

// Lifecycle controls the lifecycle of service depending on fraud proofs.
// It starts the service only if no fraud-proof exists and stops the service automatically
// if a proof arrives after the service was started.
func Lifecycle(
	startCtx, lifecycleCtx context.Context,
	p fraud.ProofType,
	fraudServ fraud.Service,
	start, stop func(context.Context) error,
) error {
	proofs, err := fraudServ.Get(startCtx, p)
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
	go fraud.OnProof(lifecycleCtx, fraudServ, p, func(fraud.Proof) {
		ctx, cancel := context.WithTimeout(lifecycleCtx, time.Minute)
		defer cancel()
		if err := stop(ctx); err != nil {
			log.Error(err)
		}
	})
	return nil
}
