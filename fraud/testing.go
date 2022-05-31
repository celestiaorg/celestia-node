package fraud

import "context"

type DummyService struct {
}

func (d *DummyService) Start(context.Context) error {
	return nil
}

func (d *DummyService) Stop(context.Context) error {
	return nil
}

func (d *DummyService) Broadcast(context.Context, Proof) error {
	return nil
}

func (d *DummyService) Subscribe(ProofType) (Subscription, error) {
	return &dummySubscription{}, nil
}

func (d *DummyService) RegisterUnmarshaler(ProofType, ProofUnmarshaler) error {
	return nil
}

func (d *DummyService) UnregisterUnmarshaler(ProofType) error {
	return nil
}

func (d *DummyService) AddValidator(ProofType, Validator) error {
	return nil
}

type dummySubscription struct {
}

func (d *dummySubscription) Proof(ctx context.Context) (Proof, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (d *dummySubscription) Cancel() {}
