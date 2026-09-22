package canary

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/stretchr/testify/require"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/telemetry"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

type simSession struct {
	heightReads    int
	localHashReads []string
	mu             sync.Mutex
	t              *testing.T
	suite          *headertest.TestSuite
	hs             []*header.ExtendedHeader
	c              *telemetry.Collector
	info           model.ProcessInfo
	epoch          int
	headCalls      int
	reader         *io.PipeReader
	writer         *io.PipeWriter
	done           chan struct{}
	cleaned        bool
	// Negative controls: no sampling at all, no live head after startup, only
	// an empty live head, no sampling after restart, identity drift, a forced
	// stop and a cleanup failure.
	noSample, noRecent, emptyRecent, noPost, drift, forced, cleanupFailure bool
	// emptyFirst makes the first completion an empty block: the availability
	// check returns before any session, and the DAS worker still logs the
	// completion.
	emptyFirst bool
	// crashFirst kills the node once the first sampling wait queries it (use
	// with noSample so no witness can precede the death); crashRestart kills it
	// right after the restart. crashed marks the death: RPC then fails like a
	// connection to a dead process.
	crashFirst, crashRestart, crashed bool
}

func simPeer() peer.ID { return peer.ID("\x12\x20" + strings.Repeat("u", 32)) }

func newSimulation(t *testing.T) (*simSession, model.Profile) {
	p := localProfile()
	p.Bootstrappers = []string{"/ip4/127.0.0.1/tcp/2121/p2p/" + simPeer().String()}
	suite := headertest.NewTestSuite(
		t,
		headertest.WithStartTime(time.Now().Add(-10*time.Second)),
		headertest.WithBlockTime(time.Millisecond),
	)
	s := &simSession{t: t, suite: suite, hs: suite.GenExtendedHeaders(101), c: collectorFor(t, p)}
	s.withData(s.hs[100])
	s.info = model.ProcessInfo{
		ContainerID: "container",
		VolumeID:    "volume",
		ImageID:     "sha256:" + strings.Repeat("b", 64),
	}
	return s, p
}

// withData gives a synthetic header a real non-empty data square.
func (s *simSession) withData(h *header.ExtendedHeader) {
	roots, err := share.NewAxisRoots(edstest.RandEDS(s.t, 4))
	require.NoError(s.t, err)
	h.DAH = roots
	h.DataHash = roots.Hash()
	h.Commit = s.suite.Commit(&h.RawHeader)
}

// withEmpty gives a synthetic header the empty data square.
func (s *simSession) withEmpty(h *header.ExtendedHeader) {
	h.DAH = share.EmptyEDSRoots()
	h.DataHash = h.DAH.Hash()
	h.Commit = s.suite.Commit(&h.RawHeader)
}

// bootstrapHead returns the head the node bootstrapped at. The emitter
// goroutines read it while a test may grow hs through appendHeader.
func (s *simSession) bootstrapHead() *header.ExtendedHeader {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hs[100]
}

// appendHeader mints the next header at the current time; empty leaves the EDS empty.
func (s *simSession) appendHeader(empty bool) *header.ExtendedHeader {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.hs[len(s.hs)-1]
	raw := s.suite.GenRawHeader(prev.Height()+1, prev.Hash(), libhead.Hash(prev.Commit.Hash()), prev.DAH.Hash())
	raw.Time = time.Now().UTC()
	h := &header.ExtendedHeader{
		RawHeader:    *raw,
		Commit:       s.suite.Commit(raw),
		ValidatorSet: prev.ValidatorSet,
		DAH:          prev.DAH,
	}
	if empty {
		s.withEmpty(h)
	} else {
		s.withData(h)
	}
	s.hs = append(s.hs, h)
	return h
}

func (s *simSession) Start(context.Context) (model.ProcessInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reader == nil {
		return s.info, errors.New("must preattach")
	}
	s.epoch++
	s.headCalls = 0
	s.info.Running = true
	s.info.StartedAt = time.Now().UTC()
	s.info.FinishedAt = time.Time{}
	s.done = make(chan struct{})
	if s.crashRestart && s.epoch == 2 {
		go s.crash()
	}
	return s.info, nil
}

// crash ends the node's output and then reports the exit, the order in which
// Docker observes a node that dies on its own.
func (s *simSession) crash() {
	s.mu.Lock()
	if s.crashed {
		s.mu.Unlock()
		return
	}
	s.crashed = true
	w := s.writer
	s.mu.Unlock()
	_ = w.Close()
	s.mu.Lock()
	s.info.Running = false
	s.info.FinishedAt = time.Now().UTC()
	s.info.ExitCode = 1
	s.mu.Unlock()
}

func (s *simSession) Stop(context.Context) (model.ProcessInfo, error) {
	s.mu.Lock()
	w := s.writer
	done := s.done
	s.mu.Unlock()
	select {
	case <-done:
	case <-time.After(time.Second):
	}
	w.Close()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.info.Running = false
	s.info.FinishedAt = time.Now().UTC()
	s.info.ForcedKill = s.forced
	return s.info, nil
}

func (s *simSession) Inspect(context.Context) (model.ProcessInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.info, nil
}

func (s *simSession) Logs(context.Context) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.info.Running {
		return nil, errors.New("attach must precede start")
	}
	s.reader, s.writer = io.Pipe()
	return s.reader, nil
}

func (s *simSession) RPC(context.Context) (Reader, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.crashed {
		return nil, errors.New("connection refused")
	}
	return s, nil
}

func (*simSession) Store() model.StoreIdentity {
	return model.StoreIdentity{Fresh: true, VolumeID: "volume", Owner: "run-1"}
}

func (s *simSession) Cleanup(ctx context.Context) error {
	s.mu.Lock()
	s.cleaned = ctx.Err() == nil
	r, w := s.reader, s.writer
	fail := s.cleanupFailure
	s.mu.Unlock()
	if r != nil {
		r.Close()
		w.Close()
	}
	if fail {
		return errors.New("cleanup failed")
	}
	return nil
}
func (*simSession) Close() error { return nil }
func (s *simSession) Info(context.Context) (peer.AddrInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := simPeer()
	if s.drift && s.epoch == 2 {
		id = peer.ID("changed")
	}
	return peer.AddrInfo{ID: id}, nil
}
func (*simSession) Peers(context.Context) ([]peer.ID, error) { return []peer.ID{simPeer()}, nil }
func (s *simSession) SamplingStats(context.Context) (das.SamplingStats, error) {
	s.mu.Lock()
	crash := s.crashFirst && s.epoch == 1
	s.mu.Unlock()
	if crash {
		go s.crash()
	}
	return das.SamplingStats{}, nil // deliberately no frontier progress
}

func (s *simSession) NetworkHead(context.Context) (*header.ExtendedHeader, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.headCalls++
	if s.epoch == 1 && s.headCalls == 1 {
		go s.emitEpochOne(s.writer, s.done)
	}
	// After restart: readiness calls once, the resumed-sampling observer twice.
	if s.epoch == 2 && s.headCalls == 2 {
		go s.emitEpochTwo(s.writer, s.done)
	}
	return s.hs[len(s.hs)-1], nil
}

func (s *simSession) LocalHead(context.Context) (*header.ExtendedHeader, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hs[len(s.hs)-1], nil
}

func (s *simSession) Tail(context.Context) (*header.ExtendedHeader, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hs[0], nil
}

func (s *simSession) GetByHeight(_ context.Context, h uint64) (*header.ExtendedHeader, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.heightReads++
	for _, x := range s.hs {
		if x.Height() == h {
			return x, nil
		}
	}
	return nil, libhead.ErrNotFound
}

func (s *simSession) GetByHash(_ context.Context, h libhead.Hash) (*header.ExtendedHeader, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.localHashReads = append(s.localHashReads, h.String())
	if !s.info.Running {
		return nil, errors.New("native RPC unavailable after stop")
	}
	for _, x := range s.hs {
		if x.Hash().String() == h.String() {
			return x, nil
		}
	}
	return nil, libhead.ErrNotFound
}

func (s *simSession) GetRangeByHeight(
	_ context.Context,
	from *header.ExtendedHeader,
	to uint64,
) ([]*header.ExtendedHeader, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if to-from.Height()-1 > 64 {
		return nil, libhead.ErrHeadersLimitExceeded
	}
	var out []*header.ExtendedHeader
	for _, x := range s.hs {
		if x.Height() > from.Height() && x.Height() < to {
			out = append(out, x)
		}
	}
	return out, nil
}

type muxFixture struct {
	io.Writer
	stream stdcopy.StdType
}

func (m muxFixture) Write(p []byte) (int, error) {
	var frame [8]byte
	frame[0] = byte(m.stream)
	binary.BigEndian.PutUint32(frame[4:], uint32(len(p)))
	if _, err := m.Writer.Write(frame[:]); err != nil {
		return 0, err
	}
	return m.Writer.Write(p)
}

func nativeRecord(h *header.ExtendedHeader, at time.Time, kind, jobType string) []byte {
	m := map[string]any{"ts": at.UTC().Format(time.RFC3339Nano), "caller": "synthetic/unit.go:1"}
	if kind == "start" {
		m["logger"], m["level"], m["msg"], m["root"] = "share/light", "debug", "starting sampling session", h.DAH.String()
	} else {
		level := "debug"
		if jobType == "recent" {
			level = "info"
		}
		m["logger"], m["level"], m["msg"], m["type"] = "das", level, "sampled header", jobType
		m["height"], m["hash"], m["data root"] = h.Height(), h.Hash().String(), h.DAH.String()
		m["EDS square width"], m["finished (s)"] = len(h.DAH.RowRoots), 0.01
	}
	raw, _ := json.Marshal(m)
	return append(raw, '\n')
}

// sample writes a complete sampling session for h: start, then completion.
// Timestamps are real wall-clock times, never ahead of the engine's clock.
func sample(w io.Writer, h *header.ExtendedHeader, jobType string) {
	_, _ = w.Write(nativeRecord(h, time.Now().UTC().Truncate(time.Millisecond), "start", ""))
	time.Sleep(2 * time.Millisecond)
	_, _ = w.Write(nativeRecord(h, time.Now().UTC().Truncate(time.Millisecond), "success", jobType))
}

func (s *simSession) emitEpochOne(w *io.PipeWriter, done chan struct{}) {
	defer close(done)
	stderr := muxFixture{w, stdcopy.Stderr}
	time.Sleep(3 * time.Millisecond)
	irrelevant, _ := json.Marshal(
		map[string]any{
			"ts":     time.Now().UTC().Format(time.RFC3339Nano),
			"level":  "info",
			"logger": "p2p",
			"msg":    "synthetic fixture alive",
		},
	)
	stderr.Write(append(irrelevant, '\n'))
	stderr.Write([]byte("The p2p host is listening on:\n"))
	if s.noSample {
		return
	}
	// Catch-up sample of the bootstrap head: the first DAS completion.
	if s.emptyFirst {
		stderr.Write(nativeRecord(s.bootstrapHead(), time.Now().UTC().Truncate(time.Millisecond), "success", "catchup"))
	} else {
		sample(stderr, s.bootstrapHead(), "catchup")
	}
	if s.noRecent {
		return
	}
	time.Sleep(20 * time.Millisecond)
	live := s.appendHeader(s.emptyRecent)
	if s.emptyRecent {
		return // Empty blocks are never sampled; no session is emitted.
	}
	time.Sleep(3 * time.Millisecond)
	sample(stderr, live, "recent")
}

func (s *simSession) emitEpochTwo(w *io.PipeWriter, done chan struct{}) {
	defer close(done)
	stderr := muxFixture{w, stdcopy.Stderr}
	time.Sleep(3 * time.Millisecond)
	irrelevant, _ := json.Marshal(
		map[string]any{
			"ts":     time.Now().UTC().Format(time.RFC3339Nano),
			"level":  "info",
			"logger": "p2p",
			"msg":    "synthetic fixture restarted",
		},
	)
	stderr.Write(append(irrelevant, '\n'))
	if s.noPost {
		return
	}
	resumed := s.appendHeader(false)
	time.Sleep(3 * time.Millisecond)
	sample(stderr, resumed, "catchup")
}

func simulationOptions() RunOptions {
	return RunOptions{
		RunID:            "run-1",
		RunTimeout:       10 * time.Second,
		BootstrapTimeout: 2 * time.Second,
		HeaderTimeout:    2 * time.Second,
		SamplingTimeout:  700 * time.Millisecond,
		RecentTimeout:    700 * time.Millisecond,
		RestartTimeout:   2 * time.Second,
		CleanupTimeout:   time.Second,
		PollInterval:     time.Millisecond,
		ExitSettle:       200 * time.Millisecond,
	}
}

func TestRunSessionPassesWithNativeLogWitnesses(t *testing.T) {
	s, p := newSimulation(t)
	r, err := RunSession(context.Background(), p, simulationOptions(), s, s.c)
	require.NoError(t, err)
	require.Equal(t, model.Pass, r.Outcome, "%+v diagnostics=%+v", r.Checks, s.c.Diagnostics())
	require.NoError(t, r.Validate())
	require.True(t, s.cleaned)
	require.Equal(t, 2, s.epoch)
	require.Len(t, r.Witnesses, 3)
	require.Equal(t, s.hs[100].Hash().String(), r.Witnesses[0].Header.Hash)
	require.Equal(t, "catchup", r.Witnesses[0].JobType)
	require.Equal(t, "recent", r.Witnesses[1].JobType)
	require.Equal(t, s.hs[100].Height()+1, r.Witnesses[1].Header.Height)
	require.Equal(t, 2, r.Witnesses[2].Epoch)
	require.Equal(t, 100, checkNamed(r, "header_sync").Evidence["count"])
	require.Equal(t, ref(s.hs[100]), checkNamed(r, "restart").Evidence["retained_header"])
	require.Zero(t, s.heightReads, "DAS observer must not walk historical heights")
	for _, name := range model.WitnessChecks() {
		require.Equal(t, model.Pass, checkNamed(r, name).Outcome, name)
		require.Contains(t, checkNamed(r, name).Evidence, "witness")
		require.Contains(t, checkNamed(r, name).Evidence, "terminal_collector")
	}
	for _, m := range []*float64{
		r.Metrics.StartupSeconds,
		r.Metrics.HeaderSyncSeconds,
		r.Metrics.FirstSampleSeconds,
		r.Metrics.RecentSampleSeconds,
		r.Metrics.RestartResumeSeconds,
	} {
		require.NotNil(t, m)
		require.GreaterOrEqual(t, *m, 0.0)
	}
	require.Equal(t, 1, s.c.Diagnostics()["unparsed_lines"], "plain banner line must be counted, not fatal")
}

func TestLiveHeadIsSoft(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*simSession)
		code  string
	}{
		{"no live head", func(s *simSession) { s.noRecent = true }, "no_new_head_observed"},
		{"empty live head", func(s *simSession) { s.emptyRecent = true }, "no_eligible_nonempty_block"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, p := newSimulation(t)
			tc.setup(s)
			r, err := RunSession(context.Background(), p, simulationOptions(), s, s.c)
			require.NoError(t, err)
			require.NoError(t, r.Validate())
			require.Equal(t, model.Pass, r.Outcome, "%+v", r.Checks)
			recent := checkNamed(r, "das_recent")
			require.Equal(t, model.Inconclusive, recent.Outcome)
			require.Equal(t, tc.code, recent.Code)
			require.Equal(t, true, recent.Evidence["soft_check"])
			require.Nil(t, r.Metrics.RecentSampleSeconds)
			require.Len(t, r.Witnesses, 2)
			require.Equal(t, []string{"das", "restart"}, []string{r.Witnesses[0].Check, r.Witnesses[1].Check})
			require.NotNil(t, r.Metrics.RestartResumeSeconds)
		})
	}
}

func TestEmptyFirstBlockCompletesWithoutSession(t *testing.T) {
	s, p := newSimulation(t)
	s.emptyFirst = true
	s.withEmpty(s.hs[100])
	r, err := RunSession(context.Background(), p, simulationOptions(), s, s.c)
	require.NoError(t, err)
	require.Equal(t, model.Pass, r.Outcome, "%+v diagnostics=%+v", r.Checks, s.c.Diagnostics())
	require.NoError(t, r.Validate())
	das := checkNamed(r, "das")
	require.Equal(t, "native_empty_block_completed", das.Code)
	require.Equal(t, true, das.Evidence["empty_block"])
	require.Contains(t, das.Evidence, "terminal_collector")
	require.Len(t, r.Witnesses, 3)
	require.True(t, r.Witnesses[0].Empty)
	require.Zero(t, r.Witnesses[0].SampleCount)
	require.Equal(t, s.hs[100].Hash().String(), r.Witnesses[0].Header.Hash)
	require.Equal(t, "catchup", r.Witnesses[0].JobType)
	require.False(t, r.Witnesses[1].Empty)
	require.False(t, r.Witnesses[2].Empty)
	require.Equal(t, "sampling_resumed", checkNamed(r, "restart").Code)
	require.NotNil(t, r.Metrics.FirstSampleSeconds)
	require.Equal(
		t,
		1,
		s.c.Diagnostics()["sampled_headers"].(int)-s.c.Diagnostics()["sampling_sessions"].(int),
		"exactly one completion without a session",
	)
}

func TestNodeCrashFailsTheWaitingPhaseWithoutVoidingEarlierEvidence(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		setup                func(*simSession)
		das, recent, restart model.Outcome
		code                 string
	}{
		{
			"during first sampling", func(s *simSession) { s.noSample, s.crashFirst = true, true },
			model.Fail, model.Inconclusive, model.Inconclusive, "das=process_exited",
		},
		{
			"on restart", func(s *simSession) { s.crashRestart = true },
			model.Pass, model.Pass, model.Fail, "restart=process_exited",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, p := newSimulation(t)
			tc.setup(s)
			r, err := RunSession(context.Background(), p, simulationOptions(), s, s.c)
			require.NoError(t, err)
			require.Equal(t, model.Fail, r.Outcome)
			require.Equal(t, tc.das, checkNamed(r, model.CheckDAS).Outcome, "das")
			require.Equal(t, tc.recent, checkNamed(r, model.CheckDASRecent).Outcome, "das_recent")
			require.Equal(t, tc.restart, checkNamed(r, model.CheckRestart).Outcome, "restart")
			name, code, _ := strings.Cut(tc.code, "=")
			failed := checkNamed(r, name)
			require.Equal(t, code, failed.Code)
			process, ok := failed.Evidence["process"].(model.ProcessInfo)
			require.True(t, ok, "failed check carries the process state")
			require.Equal(t, 1, process.ExitCode)
			require.False(t, process.Running)
			// A node that died is a node failure, not lost evidence.
			require.Equal(t, false, s.c.Diagnostics()["evidence_gap"])
		})
	}
}
