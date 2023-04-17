package fraud

import (
	"context"
	"fmt"

	"github.com/ipfs/go-datastore"

	"github.com/celestiaorg/go-fraud"
)

// service defines minimal interface with service lifecycle methods
type service interface {
	Start(context.Context) error
	Stop(context.Context) error
}

// ServiceBreaker wraps any service with fraud proof subscription of a specific type.
// If proof happens the service is Stopped automatically.
// TODO(@Wondertan): Support multiple fraud types.
type ServiceBreaker[S service] struct {
	Service   S
	FraudType fraud.ProofType
	FraudServ fraud.Service

	ctx    context.Context
	cancel context.CancelFunc
	sub    fraud.Subscription
}

// Start starts the inner service if there are no fraud proofs stored.
// Subscribes for fraud and stops the service whenever necessary.
func (breaker *ServiceBreaker[S]) Start(ctx context.Context) error {
	proofs, err := breaker.FraudServ.Get(ctx, breaker.FraudType)
	switch err {
	default:
		return fmt.Errorf("getting proof(%s): %w", breaker.FraudType, err)
	case nil:
		return &fraud.ErrFraudExists{Proof: proofs}
	case datastore.ErrNotFound:
	}

	err = breaker.Service.Start(ctx)
	if err != nil {
		return err
	}

	breaker.sub, err = breaker.FraudServ.Subscribe(breaker.FraudType)
	if err != nil {
		return fmt.Errorf("subscribing for proof(%s): %w", breaker.FraudType, err)
	}

	breaker.ctx, breaker.cancel = context.WithCancel(context.Background())
	go breaker.awaitProof()
	return nil
}

// Stop stops the service and cancels subscription.
func (breaker *ServiceBreaker[S]) Stop(ctx context.Context) error {
	if breaker.ctx.Err() != nil {
		// short circuit if the service was already stopped
		return nil
	}

	breaker.sub.Cancel()
	breaker.cancel()
	return breaker.Service.Stop(ctx)
}

func (breaker *ServiceBreaker[S]) awaitProof() {
	_, err := breaker.sub.Proof(breaker.ctx)
	if err != nil {
		return
	}

	if err := breaker.Stop(breaker.ctx); err != nil {
		log.Error(err)
	}
}
