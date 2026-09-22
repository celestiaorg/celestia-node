package canary

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
)

func TestObservedHeadersRejectInvalidNativeHeader(t *testing.T) {
	for _, kind := range []string{"chain", "signature"} {
		t.Run(kind, func(t *testing.T) {
			h := headertest.NewTestSuite(t).GenExtendedHeaders(1)[0]
			s := newObservedHeaders("test")
			if kind == "chain" {
				s = newObservedHeaders("other-chain")
			} else {
				h.Commit.Signatures[0].Signature[0] ^= 1
			}
			require.Error(t, s.observe(h))
			_, err := s.GetByHash(context.Background(), h.Hash())
			require.Error(t, err)
		})
	}
}

func TestObservedHeadersConflictIsSticky(t *testing.T) {
	a := headertest.NewTestSuite(t).GenExtendedHeaders(1)[0]
	b := headertest.NewTestSuite(t).GenExtendedHeaders(1)[0]
	require.Equal(t, a.Height(), b.Height())
	require.NotEqual(t, a.Hash(), b.Hash())
	s := newObservedHeaders("test")
	require.NoError(t, s.observe(a))
	require.Error(t, s.observe(b))
	require.Error(t, s.observe(a), "cannot restore trust after conflict")
	_, err := s.GetByHash(context.Background(), a.Hash())
	require.Error(t, err, "previous entry cannot hide a conflict")
	require.NotErrorIs(t, err, libhead.ErrNotFound)
}

func TestObservedHeadersCountLimitDoesNotEvict(t *testing.T) {
	hs := headertest.NewTestSuite(t).GenExtendedHeaders(513)
	s := newObservedHeaders("test")
	for _, h := range hs[:512] {
		require.NoError(t, s.observe(h))
	}
	for i := 0; i < 3; i++ {
		require.NoError(t, s.observe(hs[0]), "duplicate is idempotent at capacity")
	}
	require.Error(t, s.observe(hs[512]))
	require.Error(t, s.observe(hs[0]))
	_, err := s.GetByHash(context.Background(), hs[0].Hash())
	require.Error(t, err)
	require.NotErrorIs(t, err, libhead.ErrNotFound, "exhaustion is sticky, not eviction")
}

func TestObservedHeadersByteLimitsAreSticky(t *testing.T) {
	for _, kind := range []string{"single", "total"} {
		t.Run(kind, func(t *testing.T) {
			suite := headertest.NewTestSuite(t)
			s := newObservedHeaders("test")
			total := 0
			for i := 0; i < 20; i++ {
				h := suite.GenExtendedHeaders(1)[0]
				// Grow the header through AppHash, which Comet allows at any size, and
				// re-sign it.
				size := 512 << 10
				if kind == "single" {
					size = 1 << 20
				}
				h.AppHash = make([]byte, size)
				h.Commit = suite.Commit(&h.RawHeader)
				require.NoError(t, validateHeader(h, "test"))
				raw, err := h.MarshalBinary()
				require.NoError(t, err)
				total += len(raw)
				if len(raw) > 1<<20 || total > 8<<20 {
					require.Error(t, s.observe(h), "must reject before exceeding byte budget")
					require.Error(t, s.observe(h), "byte exhaustion is sticky")
					_, err = s.GetByHash(context.Background(), h.Hash())
					require.Error(t, err)
					require.NotErrorIs(t, err, libhead.ErrNotFound)
					return
				}
				require.NoError(t, s.observe(h))
				require.NoError(t, s.observe(h), "duplicates do not consume bytes")
			}
			t.Fatal("fixture did not reach byte boundary")
		})
	}
}

func TestObservedHeadersCanceledLookup(t *testing.T) {
	h := headertest.NewTestSuite(t).GenExtendedHeaders(1)[0]
	s := newObservedHeaders("test")
	require.NoError(t, s.observe(h))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := s.GetByHash(ctx, h.Hash())
	require.ErrorIs(t, err, context.Canceled)
}

func TestObservedHeadersReturnedMutationCannotChangeSnapshot(t *testing.T) {
	h := headertest.NewTestSuite(t).GenExtendedHeaders(1)[0]
	s := newObservedHeaders("test")
	require.NoError(t, s.observe(h))
	got, err := s.GetByHash(context.Background(), h.Hash())
	require.NoError(t, err)
	got.RawHeader.ChainID = "mutated-return"
	got.Commit.Signatures[0].Signature[0] ^= 1
	got.DAH.RowRoots[0][0] ^= 1
	again, err := s.GetByHash(context.Background(), h.Hash())
	require.NoError(t, err)
	require.NoError(t, validateHeader(again, "test"))
	require.Equal(t, h.Hash(), again.Hash())
}

func TestObservedHeadersValidateDetachedEncoding(t *testing.T) {
	original := headertest.NewTestSuite(t).GenExtendedHeaders(1)[0]
	raw, err := original.MarshalBinary()
	require.NoError(t, err)
	h := new(header.ExtendedHeader)
	require.NoError(t, h.UnmarshalBinary(raw)) // do not mutate shared EmptyEDSRoots fixture

	// The DAH caches its hash: mutating it after Hash() must not let the
	// previously validated pointer pass with invalid bytes.
	h.DAH.Hash()
	h.DAH.RowRoots[0][0] ^= 1
	s := newObservedHeaders("test")
	require.Error(t, s.observe(h))
	_, err = s.GetByHash(context.Background(), h.Hash())
	require.Error(t, err)
}

func TestObservedHeadersDetachNativeInput(t *testing.T) {
	h := headertest.NewTestSuite(t).GenExtendedHeaders(1)[0]
	original := ref(h)
	hash := append(libhead.Hash(nil), h.Hash()...)
	s := newObservedHeaders(h.ChainID())
	require.NoError(t, s.observe(h))
	h.RawHeader.ChainID = "mutated-input"
	got, err := s.GetByHash(context.Background(), hash)
	require.NoError(t, err)
	require.NoError(t, validateHeader(got, original.ChainID))
	require.True(t, got.Time().Equal(original.Time))
	require.Equal(t, original.Hash, got.Hash().String())
	_, err = s.GetByHash(context.Background(), libhead.Hash(make([]byte, 32)))
	require.ErrorIs(t, err, libhead.ErrNotFound)
}
