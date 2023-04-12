package fraudtest

import (
	"context"

	"github.com/celestiaorg/celestia-node/libs/fraud"
)

type DummyService struct{}

func (d *DummyService) Broadcast(context.Context, fraud.Proof) error {
	return nil
}

func (d *DummyService) Subscribe(fraud.ProofType) (fraud.Subscription, error) {
	return &subscription{}, nil
}

func (d *DummyService) Get(context.Context, fraud.ProofType) ([]fraud.Proof, error) {
	return nil, nil
}

type subscription struct{}

func (s *subscription) Proof(context.Context) (fraud.Proof, error) {
	return nil, nil
}

func (s *subscription) Cancel() {}
