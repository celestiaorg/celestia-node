package canary

import (
	"context"
	"errors"
	"sync"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
)

// observedHeaders keeps the network heads seen during one phase so that witness
// hashes can be resolved against them. Headers are stored as binary snapshots,
// which neither the caller nor a reader can modify. Limits: 512 headers, 8 MiB
// in total, 1 MiB per header. Nothing is evicted: hitting a limit or seeing
// two headers at one height fails the phase.
const (
	observedHeaderCountLimit = 512
	observedHeaderBytesLimit = 8 << 20
	observedHeaderSizeLimit  = 1 << 20
)

type observedHeaders struct {
	bytes   int
	mu      sync.Mutex
	chainID string
	headers map[string][]byte
	heights map[uint64]string
	err     error
}

func newObservedHeaders(chainID string) *observedHeaders {
	return &observedHeaders{chainID: chainID, headers: make(map[string][]byte), heights: make(map[uint64]string)}
}

func (s *observedHeaders) observe(h *header.ExtendedHeader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	if err := validateHeader(h, s.chainID); err != nil {
		return err
	}
	raw, err := h.MarshalBinary()
	if err != nil {
		return err
	}
	if len(raw) > observedHeaderSizeLimit {
		s.err = errors.New("observed native header byte limit exceeded")
		return s.err
	}
	// Validate the decoded snapshot as well: a header object caches derived
	// hashes, so a header mutated after hashing could pass on its own.
	detached := new(header.ExtendedHeader)
	if err = detached.UnmarshalBinary(raw); err != nil {
		return err
	}
	if err = validateHeader(detached, s.chainID); err != nil {
		return err
	}
	h = detached
	key := h.Hash().String()
	if previous, ok := s.heights[h.Height()]; ok && previous != key {
		s.err = errors.New("conflicting observed native header at height")
		return s.err
	}
	if _, ok := s.headers[key]; ok {
		return nil
	}
	if len(s.headers) >= observedHeaderCountLimit {
		s.err = errors.New("observed native header count limit exceeded")
		return s.err
	}
	if s.bytes > observedHeaderBytesLimit-len(raw) {
		s.err = errors.New("observed native header byte limit exceeded")
		return s.err
	}
	s.bytes += len(raw)
	s.heights[h.Height()] = key
	s.headers[key] = raw
	return nil
}

func (s *observedHeaders) GetByHash(ctx context.Context, hash libhead.Hash) (*header.ExtendedHeader, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
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
