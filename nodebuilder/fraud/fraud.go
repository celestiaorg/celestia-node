package fraud

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/celestiaorg/go-fraud"

	"github.com/celestiaorg/celestia-node/header"
)

var _ Module = (*API)(nil)

// Module encompasses the behavior necessary to subscribe and broadcast fraud proofs within the
// network. Any method signature changed here needs to also be changed in the API struct.
//
//go:generate mockgen -destination=mocks/api.go -package=mocks . Module
type Module interface {
	// Subscribe allows to subscribe on a Proof pub sub topic by its type.
	Subscribe(context.Context, fraud.ProofType) (<-chan *Proof, error)
	// Get fetches fraud proofs from the disk by its type.
	Get(context.Context, fraud.ProofType) ([]Proof, error)
}

// API is a wrapper around Module for the RPC.
type API struct {
	Internal struct {
		Subscribe func(context.Context, fraud.ProofType) (<-chan *Proof, error) `perm:"read"`
		Get       func(context.Context, fraud.ProofType) ([]Proof, error)       `perm:"read"`
	}
}

func (api *API) Subscribe(ctx context.Context, proofType fraud.ProofType) (<-chan *Proof, error) {
	return api.Internal.Subscribe(ctx, proofType)
}

func (api *API) Get(ctx context.Context, proofType fraud.ProofType) ([]Proof, error) {
	return api.Internal.Get(ctx, proofType)
}

var _ Module = (*module)(nil)

// module is an implementation of Module that uses fraud.module as a backend. It is used to
// provide fraud proofs as a non-interface type to the API, and wrap fraud.Subscriber with a
// channel of Proofs.
type module struct {
	fraud.Service[*header.ExtendedHeader]
}

func (s *module) Subscribe(ctx context.Context, proofType fraud.ProofType) (<-chan *Proof, error) {
	subscription, err := s.Service.Subscribe(proofType)
	if err != nil {
		return nil, err
	}
	proofs := make(chan *Proof)
	go func() {
		defer close(proofs)
		defer subscription.Cancel()
		for {
			proof, err := subscription.Proof(ctx)
			if err != nil {
				if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
					log.Errorw("fetching proof from subscription", "err", err)
				}
				return
			}
			select {
			case <-ctx.Done():
				return
			case proofs <- &Proof{Proof: proof}:
			}
		}
	}()
	return proofs, nil
}

func (s *module) Get(ctx context.Context, proofType fraud.ProofType) ([]Proof, error) {
	originalProofs, err := s.Service.Get(ctx, proofType)
	if err != nil {
		return nil, err
	}
	proofs := make([]Proof, len(originalProofs))
	for i, originalProof := range originalProofs {
		proofs[i].Proof = originalProof
	}
	return proofs, nil
}

// Proof embeds the fraud.Proof interface type to provide a concrete type for JSON serialization.
type Proof struct {
	fraud.Proof[*header.ExtendedHeader]
}

type fraudProofJSON struct {
	ProofType fraud.ProofType `json:"proof_type"`
	Data      []byte          `json:"data"`
}

func (f *Proof) UnmarshalJSON(data []byte) error {
	var fp fraudProofJSON
	err := json.Unmarshal(data, &fp)
	if err != nil {
		return err
	}
	f.Proof, err = defaultProofUnmarshaler.Unmarshal(fp.ProofType, fp.Data)
	return err
}

func (f *Proof) MarshalJSON() ([]byte, error) {
	marshaledProof, err := f.MarshalBinary()
	if err != nil {
		return nil, err
	}
	fraudProof := &fraudProofJSON{
		ProofType: f.Type(),
		Data:      marshaledProof,
	}
	return json.Marshal(fraudProof)
}
