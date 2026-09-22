package canary

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/telemetry"
)

type phaseHeadReader struct {
	*simSession
	heads []*header.ExtendedHeader
	calls int
	phase context.Context
}

func (r *phaseHeadReader) LocalHead(ctx context.Context) (*header.ExtendedHeader, error) {
	return r.NetworkHead(ctx)
}

func (r *phaseHeadReader) NetworkHead(ctx context.Context) (*header.ExtendedHeader, error) {
	r.phase = ctx
	h := r.heads[min(r.calls, len(r.heads)-1)]
	r.calls++
	return h, nil
}

func phaseHeadFixture(t *testing.T, noSample bool) (*simSession, model.Profile, *logCapture) {
	t.Helper()
	s, p := newSimulation(t)
	s.noSample = noSample
	s.noRecent = true
	stream, err := s.Logs(context.Background())
	require.NoError(t, err)
	info, err := s.Start(context.Background())
	require.NoError(t, err)
	require.NoError(t, s.c.BeginEpoch(telemetry.Epoch{Number: 1, StartedAt: info.StartedAt}))
	pi, err := s.Info(context.Background())
	require.NoError(t, err)
	require.NoError(t, s.c.SetNodeID(pi.ID.String()))
	cap := captureLogs(context.Background(), stream, s.c, 1, nil, 0)
	t.Cleanup(func() { cap.close(); require.NoError(t, s.Cleanup(context.Background())) })
	go s.emitEpochOne(s.writer, s.done)
	return s, p, cap
}

func TestSamplingWitnessResolvesThroughObservedHeadOrLocalStore(t *testing.T) {
	s, p, cap := phaseHeadFixture(t, false)
	reader := &phaseHeadReader{simSession: s, heads: []*header.ExtendedHeader{s.hs[99]}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	snapshots := &witnessHeaders{}
	w, out, code, ev := waitSampling(
		ctx,
		reader,
		s.c,
		telemetry.WitnessRequest{Epoch: 1},
		p,
		simulationOptions(),
		cap,
		snapshots,
		phaseFirstSample,
	)
	require.Equal(t, model.Pass, out, "%s %v", code, ev)
	require.Equal(t, "native_sample_observed", code)
	require.Equal(t, s.hs[100].Hash().String(), w.Header.Hash)
	require.Len(t, snapshots.headers, 1)
	require.Contains(
		t,
		s.localHashReads,
		s.hs[100].Hash().String(),
		"unobserved head must be read back from the local store",
	)
	require.Zero(t, s.heightReads)
}

func TestLivePhaseDistinguishesMissingHeadsFromMissingSampling(t *testing.T) {
	for _, tc := range []struct {
		name  string
		heads func(s *simSession) []*header.ExtendedHeader
		code  string
		out   model.Outcome
	}{
		{
			"no newer head",
			func(s *simSession) []*header.ExtendedHeader { return []*header.ExtendedHeader{s.hs[100]} },
			"no_new_head_observed",
			model.Inconclusive,
		},
		{
			"only empty newer head",
			func(s *simSession) []*header.ExtendedHeader { return []*header.ExtendedHeader{s.appendHeader(true)} },
			"no_eligible_nonempty_block",
			model.Inconclusive,
		},
		{
			"nonempty newer head unsampled",
			func(s *simSession) []*header.ExtendedHeader { return []*header.ExtendedHeader{s.appendHeader(false)} },
			"recent_sampling_timeout",
			model.Fail,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, p, cap := phaseHeadFixture(t, false)
			reader := &phaseHeadReader{simSession: s, heads: tc.heads(s)}
			ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
			defer cancel()
			req := telemetry.WitnessRequest{Epoch: 1, JobType: "recent", MinHeight: s.hs[100].Height()}
			w, out, code, _ := waitSampling(
				ctx,
				reader,
				s.c,
				req,
				p,
				simulationOptions(),
				cap,
				&witnessHeaders{},
				phaseRecentSample,
			)
			require.Equal(t, model.Witness{}, w)
			require.Equal(t, tc.out, out)
			require.Equal(t, tc.code, code)
		})
	}
}

func TestSamplingSnapshotCaptureFailureNeverPasses(t *testing.T) {
	s, p, cap := phaseHeadFixture(t, false)
	reader := &phaseHeadReader{simSession: s, heads: []*header.ExtendedHeader{s.hs[100]}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	snapshots := &witnessHeaders{sealed: true}
	w, out, code, _ := waitSampling(
		ctx,
		reader,
		s.c,
		telemetry.WitnessRequest{Epoch: 1},
		p,
		simulationOptions(),
		cap,
		snapshots,
		phaseFirstSample,
	)
	require.Equal(t, model.Witness{}, w)
	require.Equal(t, model.Inconclusive, out)
	require.Equal(t, "native_header_snapshot_unavailable", code)
	require.Empty(t, snapshots.headers)
	require.ErrorIs(t, reader.phase.Err(), context.Canceled)
}

func TestChosenHeadSnapshotRevalidatesExactWitnessAfterStop(t *testing.T) {
	s, p, cap := phaseHeadFixture(t, false)
	reader := &phaseHeadReader{simSession: s, heads: []*header.ExtendedHeader{s.hs[100]}}
	snapshots := &witnessHeaders{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req := telemetry.WitnessRequest{Epoch: 1}
	w, out, code, _ := waitSampling(ctx, reader, s.c, req, p, simulationOptions(), cap, snapshots, phaseFirstSample)
	require.Equal(t, model.Pass, out, code)
	require.Len(t, snapshots.headers, 1)
	snapshots.sealed = true
	cap.expected.Store(true)
	stopped, err := s.Stop(ctx)
	require.NoError(t, err)
	cap.drain(ctx)
	require.NoError(t, s.c.EndEpoch(1, stopped.FinishedAt))
	s.hs[100].RawHeader.ChainID = "mutated-after-stop"
	reads := len(s.localHashReads)
	confirmed, err := s.c.WaitWitness(ctx, req, snapshots)
	require.NoError(t, err)
	require.True(t, sameWitness(confirmed, w))
	require.Len(t, s.localHashReads, reads, "final verification may not use stopped RPC")
	other := s.suite.GenExtendedHeaders(1)[0]
	snapshots.headers[w.Header.Hash], err = other.MarshalBinary()
	require.NoError(t, err)
	rejectCtx, rejectCancel := context.WithTimeout(ctx, 25*time.Millisecond)
	defer rejectCancel()
	_, err = s.c.WaitWitness(rejectCtx, req, snapshots)
	require.Error(t, err, "a different valid native snapshot cannot replace the exact selected witness")
}

// exitedSession is a node whose process stopped on its own: RPC never answers.
type exitedSession struct {
	Session
	info model.ProcessInfo
}

func (s exitedSession) RPC(context.Context) (Reader, error) {
	return nil, errors.New("connection refused")
}

func (s exitedSession) Inspect(context.Context) (model.ProcessInfo, error) { return s.info, nil }

func TestReadinessStopsWhenTheNodeProcessExits(t *testing.T) {
	p := localProfile()
	p.Bootstrappers = []string{"/ip4/127.0.0.1/tcp/2121/p2p/" + simPeer().String()}
	s := exitedSession{info: model.ProcessInfo{ContainerID: "container", FinishedAt: time.Now().UTC(), ExitCode: 1}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	started := time.Now()
	reader, _, _, _, err := ready(ctx, s, p, simulationOptions())
	require.Nil(t, reader)
	require.ErrorIs(t, err, errProcessExited)
	require.ErrorContains(t, err, "code 1")
	require.Less(t, time.Since(started), 5*time.Second, "an exited node must not wait out the phase")

	out, code := errorOutcome(ctx, err)
	require.Equal(t, model.Fail, out)
	require.Equal(t, "process_exited", code)
	require.Equal(t, map[string]any{"process": s.info}, processEvidence(ctx, s))

	// A node that is still running keeps being polled until the phase ends.
	s.info = model.ProcessInfo{ContainerID: "container", Running: true}
	short, cancelShort := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancelShort()
	reader, _, _, _, err = ready(short, s, p, simulationOptions())
	require.Nil(t, reader)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}
