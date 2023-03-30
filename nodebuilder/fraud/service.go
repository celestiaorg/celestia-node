package fraud

import (
	"context"
	"encoding/json"

	"github.com/celestiaorg/celestia-node/libs/fraud"
)

var _ Module = (*Service)(nil)

// Service is an implementation of Module that uses fraud.Service as a backend. It is used to
// provide fraud proofs as a non-interface type to the API, and wrap fraud.Subscriber with a
// channel of Proofs.
type Service struct {
	fraud.Service
}

func (s *Service) Subscribe(ctx context.Context, proofType fraud.ProofType) (<-chan Proof, error) {
	subscription, err := s.Service.Subscribe(proofType)
	if err != nil {
		return nil, err
	}
	proofs := make(chan Proof)
	go func() {
		defer close(proofs)
		for {
			proof, err := subscription.Proof(ctx)
			if err != nil {
				if err != context.DeadlineExceeded && err != context.Canceled {
					log.Errorw("fetching proof from subscription", "err", err)
				}
				return
			}
			select {
			case <-ctx.Done():
				return
			case proofs <- Proof{Proof: proof}:
			}
		}
	}()
	return proofs, nil
}

func (s *Service) Get(ctx context.Context, proofType fraud.ProofType) ([]Proof, error) {
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
	fraud.Proof
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
	f.Proof, err = fraud.Unmarshal(fp.ProofType, fp.Data)
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
