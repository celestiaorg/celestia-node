package canary

import (
	"context"
	"encoding/hex"
	"errors"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/telemetry"
)

// witnessHeaders keeps the validated header of each selected witness (at most
// one per witness check) as a binary snapshot for the terminal re-check. Once
// sealed it does not fall back to RPC, even for unknown hashes.
const maxWitnessHeaders = 3

type witnessHeaders struct {
	headers map[string][]byte
	sealed  bool
}

func (s *witnessHeaders) capture(ctx context.Context, r telemetry.HeaderLookup, w model.Witness) error {
	if s.sealed || len(s.headers) >= maxWitnessHeaders {
		return errors.New("witness header snapshot is sealed or full")
	}
	hash, err := hex.DecodeString(w.Header.Hash)
	if err != nil || len(hash) != 32 {
		return errors.New("invalid witness header hash")
	}
	h, err := r.GetByHash(ctx, libhead.Hash(hash))
	if err != nil {
		return err
	}
	if err = validateHeader(h, w.Header.ChainID); err != nil {
		return err
	}
	got := ref(h)
	if !got.Time.Equal(w.Header.Time) {
		return errors.New("witness header timestamp changed")
	}
	got.Time = w.Header.Time
	if got != w.Header {
		return errors.New("witness header identity changed")
	}
	raw, err := h.MarshalBinary()
	if err != nil {
		return err
	}
	if len(raw) > 1<<20 {
		return errors.New("witness header snapshot exceeds limit")
	}
	if s.headers == nil {
		s.headers = make(map[string][]byte, maxWitnessHeaders)
	}
	if _, ok := s.headers[w.Header.Hash]; ok {
		return errors.New("duplicate witness header")
	}
	s.headers[w.Header.Hash] = raw
	return nil
}

func (s *witnessHeaders) GetByHash(ctx context.Context, hash libhead.Hash) (*header.ExtendedHeader, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !s.sealed {
		return nil, errors.New("witness headers are not sealed")
	}
	raw, ok := s.headers[hash.String()]
	if !ok {
		return nil, libhead.ErrNotFound
	}
	h := new(header.ExtendedHeader)
	if err := h.UnmarshalBinary(raw); err != nil {
		return nil, err
	}
	return h, nil
}

func sameWitness(a, b model.Witness) bool {
	if !a.Header.Time.Equal(b.Header.Time) {
		return false
	}
	a.Header.Time = b.Header.Time
	return a == b
}
