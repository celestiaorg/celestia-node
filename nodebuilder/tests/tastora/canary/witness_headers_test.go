package canary

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
)

func TestWitnessSnapshotsAreNativeDetachedBoundedAndSealed(t *testing.T) {
	hs := headertest.NewTestSuite(t).GenExtendedHeaders(4)
	live := &headerFixture{headers: hs}
	ctx := context.Background()
	s := &witnessHeaders{}
	original := model.Witness{Header: ref(hs[0])}
	require.NoError(t, s.capture(ctx, live, original))
	_, err := s.GetByHash(ctx, hs[0].Hash())
	require.Error(t, err, "unsealed lookup forbidden")
	require.Error(t, s.capture(ctx, live, original), "duplicate snapshot forbidden")
	require.NoError(t, s.capture(ctx, live, model.Witness{Header: ref(hs[1])}))
	require.NoError(t, s.capture(ctx, live, model.Witness{Header: ref(hs[2])}))
	require.Error(t, s.capture(ctx, live, model.Witness{Header: ref(hs[3])}), "three-header bound")
	hash := append(libhead.Hash(nil), hs[0].Hash()...)
	hs[0].RawHeader.ChainID = "mutated-live"
	s.sealed = true
	got, err := s.GetByHash(ctx, hash)
	require.NoError(t, err)
	require.NoError(t, validateHeader(got, "test"))
	require.Equal(t, original.Header.Hash, got.Hash().String())
	got.RawHeader.ChainID = "mutated-returned"
	again, err := s.GetByHash(ctx, hash)
	require.NoError(t, err)
	require.NoError(t, validateHeader(again, "test"))
	_, err = s.GetByHash(ctx, hs[3].Hash())
	require.ErrorIs(t, err, libhead.ErrNotFound)
	require.Error(t, s.capture(ctx, live, model.Witness{Header: ref(hs[3])}))
}

func TestWitnessSnapshotsRejectChangedOrInvalidNativeHeader(t *testing.T) {
	hs := headertest.NewTestSuite(t).GenExtendedHeaders(1)
	live := &headerFixture{headers: hs}
	ctx := context.Background()
	w := model.Witness{Header: ref(hs[0])}
	wrong := w
	wrong.Header.Width++
	require.Error(t, (&witnessHeaders{}).capture(ctx, live, wrong))
	wrong = w
	wrong.Header.Hash = "malformed"
	require.Error(t, (&witnessHeaders{}).capture(ctx, live, wrong))
	hs[0].RawHeader.ChainID = "changed"
	require.Error(t, (&witnessHeaders{}).capture(ctx, live, w))
}

type gapAtFinalStop struct{ *simSession }

func (s gapAtFinalStop) Stop(ctx context.Context) (model.ProcessInfo, error) {
	info, err := s.simSession.Stop(ctx)
	if s.epoch == 2 {
		s.c.MarkGap(1, "delayed old-epoch capture loss at final native flush")
	}
	return info, err
}

func TestPostStopRevalidationRetainsLifetimeNegativeEvidence(t *testing.T) {
	s, p := newSimulation(t)
	r, err := RunSession(context.Background(), p, simulationOptions(), gapAtFinalStop{s}, s.c)
	require.NoError(t, err)
	require.NoError(t, r.Validate())
	require.Equal(t, model.Inconclusive, r.Outcome)
	require.Equal(t, "evidence_changed_after_flush", checkNamed(r, "das").Code)
	require.True(t, s.cleaned)
}

func TestRunSessionNegativeOutcomesNeverPass(t *testing.T) {
	for _, tc := range []struct {
		name, check, code string
		setup             func(*simSession)
	}{
		{"sampling_disabled", "das", "sampling_timeout", func(s *simSession) { s.noSample = true }},
		{"no_postrestart_sample", "restart", "sampling_timeout", func(s *simSession) { s.noPost = true }},
		{"peer_drift", "restart", "node_id_changed", func(s *simSession) { s.drift = true }},
		{"forced_stop", "restart", "not_graceful", func(s *simSession) { s.forced = true }},
		{"cleanup_failure", "cleanup", "cleanup_failed", func(s *simSession) { s.cleanupFailure = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, p := newSimulation(t)
			tc.setup(s)
			r, err := RunSession(context.Background(), p, simulationOptions(), s, s.c)
			require.NoError(t, err)
			require.NoError(t, r.Validate())
			require.NotEqual(t, model.Pass, r.Outcome)
			require.Equal(t, tc.code, checkNamed(r, tc.check).Code, "%+v", r.Checks)
			require.True(t, s.cleaned)
		})
	}
}
