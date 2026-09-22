package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

var baseTime = time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)

const (
	rootA = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	hashA = "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
)

func newTestCollector(t *testing.T) *Collector {
	t.Helper()
	c, err := New(
		Options{ChainID: "test", NodeID: "peer", SampleAmount: 16, SamplingWindow: time.Hour, MaxRecords: 1024},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.BeginEpoch(Epoch{Number: 1, StartedAt: baseTime.Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	return c
}

func nativeStart(root string) string {
	return `{"ts":"2026-09-08T12:00:00.000Z","level":"debug","logger":"share/light",` +
		`"caller":"light/availability.go:129","msg":"starting sampling session","root":"` + root + `"}`
}

func nativeCompletion() string {
	return `{"ts":"2026-09-08T12:00:00.200Z","level":"info","logger":"das","caller":"das/worker.go:164",` +
		`"msg":"sampled header","type":"recent","height":42,"hash":"` + hashA +
		`","EDS square width":8,"data root":"` + rootA + `","finished (s)":0.2}`
}

type localHeader struct {
	h     *header.ExtendedHeader
	calls int
}

func (s *localHeader) GetByHash(_ context.Context, hash libhead.Hash) (*header.ExtendedHeader, error) {
	s.calls++
	if s.h == nil || hash.String() != s.h.Hash().String() {
		return nil, fmt.Errorf("not stored")
	}
	return s.h, nil
}

func signedHeader(t *testing.T, height uint64, roots *share.AxisRoots, ts time.Time) *header.ExtendedHeader {
	t.Helper()
	suite := headertest.NewTestSuite(t)
	prev := suite.Head()
	raw := suite.GenRawHeader(height, prev.Hash(), libhead.Hash(prev.Commit.Hash()), roots.Hash())
	raw.Time = ts
	h := &header.ExtendedHeader{RawHeader: *raw, Commit: suite.Commit(raw), ValidatorSet: prev.ValidatorSet, DAH: roots}
	if err := h.Validate(); err != nil {
		t.Fatal(err)
	}
	return h
}

func headerFixture(t *testing.T, height uint64) *header.ExtendedHeader {
	t.Helper()
	roots, err := share.NewAxisRoots(edstest.RandEDS(t, 4))
	if err != nil {
		t.Fatal(err)
	}
	return signedHeader(t, height, roots, time.Now().Add(-time.Minute))
}

func collectorForHeader(t *testing.T, h *header.ExtendedHeader) *Collector {
	t.Helper()
	c, err := New(
		Options{
			ChainID:        h.ChainID(),
			NodeID:         "peer",
			Network:        "test",
			SampleAmount:   16,
			SamplingWindow: time.Hour,
			MaxRecords:     1024,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.BeginEpoch(Epoch{1, h.Time().Add(-time.Second)}); err != nil {
		t.Fatal(err)
	}
	return c
}

func logLine(h *header.ExtendedHeader, at time.Time, kind, jobType string) []byte {
	m := map[string]any{"ts": at.UTC().Format("2006-01-02T15:04:05.000Z"), "caller": "synthetic/unit.go:1"}
	switch kind {
	case kindStart:
		m["logger"], m["level"], m["msg"], m["root"] = loggerLight, levelDebug, msgSessionStart, h.DAH.String()
	default:
		m["logger"], m["msg"], m["type"] = loggerDAS, msgSampled, jobType
		m["height"], m["hash"] = h.Height(), h.Hash().String()
		m["data root"], m["finished (s)"] = h.DAH.String(), 0.2
		m["level"] = levelDebug
		if jobType == model.JobRecent {
			m["level"] = levelInfo
		}
		m["EDS square width"] = len(h.DAH.RowRoots)
		if kind == kindFailure {
			m["level"], m["msg"], m["err"] = levelError, msgSampleFailed, "not available"
			delete(m, "EDS square width")
			m["square width"] = len(h.DAH.RowRoots)
		}
	}
	raw, _ := json.Marshal(m)
	return raw
}

func logFixture(t *testing.T, c *Collector, epoch int, h *header.ExtendedHeader, at time.Time, kind string) {
	t.Helper()
	if err := c.LogRecord(epoch, logLine(h, at, kind, model.JobRecent)); err != nil {
		t.Fatal(err)
	}
}

func noWitness(t *testing.T, c *Collector, h *header.ExtendedHeader, req WitnessRequest) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if w, err := c.WaitWitness(ctx, req, &localHeader{h: h}); err == nil {
		t.Fatalf("unexpected witness %+v", w)
	}
}

func TestNativeJSONStartAndCompletion(t *testing.T) {
	c := newTestCollector(t)
	for _, line := range []string{
		nativeStart(rootA),
		strings.Replace(nativeCompletion(), `"height":42`, `"height":9007199254740993`, 1),
	} {
		if err := c.LogRecord(1, []byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	d := c.Diagnostics()
	if d["log_records"] != 2 || d["roots"] != 1 || d["sampled_headers"] != 1 || d["sampling_sessions"] != 1 {
		t.Fatalf("ledger: %v", d)
	}
	if c.logs[1].height != 9007199254740993 || c.logs[1].jobType != model.JobRecent {
		t.Fatal("lossy native record")
	}
}

func TestNonJSONLinesAreCountedNotFatal(t *testing.T) {
	c := newTestCollector(t)
	for _, line := range []string{
		"The p2p host is listening on:",
		"*  /ip4/127.0.0.1/tcp/2121/p2p/12D3KooW",
		"",
		"2026/09/08 12:00:00 failed to sufficiently increase receive buffer size (was: 208 kiB)",
		"[1 2 3]",
	} {
		if err := c.LogRecord(1, []byte(line)); err != nil {
			t.Fatalf("plain line rejected: %v", err)
		}
	}
	if c.Diagnostics()["unparsed_lines"] != 5 || c.Diagnostics()["incompatible"] != false {
		t.Fatalf("diagnostics: %v", c.Diagnostics())
	}
	if err := c.LogRecord(1, []byte(`{"logger":"p2p","msg":"ready"}`)); err != nil || len(c.logs) != 0 {
		t.Fatal("unrelated JSON collected")
	}
}

func TestNativeJSONRejectsAmbiguousOrMalformedEvidence(t *testing.T) {
	good := nativeCompletion()
	cases := map[string]string{
		"duplicate":         strings.Replace(good, `"height":42`, `"height":41,"height":42`, 1),
		"escaped duplicate": strings.Replace(good, `"height":42`, `"height":41,"heig\u0068t":42`, 1),
		"syntax":            `{"logger":"das"`,
		"fraction":          strings.Replace(good, `"height":42`, `"height":42.0`, 1),
		"negative":          strings.Replace(good, `"height":42`, `"height":-1`, 1),
		"overflow":          strings.Replace(good, `"height":42`, `"height":18446744073709551616`, 1),
		"missing":           strings.Replace(good, `"height":42,`, ``, 1),
		"null":              strings.Replace(good, `"height":42`, `"height":null`, 1),
		"width":             strings.Replace(good, `"EDS square width":8`, `"EDS square width":0`, 1),
		"duration":          strings.Replace(good, `"finished (s)":0.2`, `"finished (s)":-0.2`, 1),
		"string duration":   strings.Replace(good, `"finished (s)":0.2`, `"finished (s)":"0.2"`, 1),
		"lowercase":         strings.ReplaceAll(good, hashA, strings.ToLower(hashA)),
		"short root":        strings.ReplaceAll(good, rootA, "AA"),
		"level":             strings.Replace(good, `"level":"info"`, `"level":"debug"`, 1),
		"job":               strings.Replace(good, `"type":"recent"`, `"type":"mystery"`, 1),
		"timestamp":         strings.Replace(good, `"ts":"2026-09-08T12:00:00.200Z"`, `"ts":123`, 1),
		"trailing":          good + ` {}`,
		"start level":       strings.Replace(nativeStart(rootA), `"level":"debug"`, `"level":"info"`, 1),
		"start root":        nativeStart("AA"),
		"msg type":          strings.Replace(nativeStart(rootA), `"msg":"starting sampling session"`, `"msg":42`, 1),
		"logger type":       strings.Replace(nativeStart(rootA), `"logger":"share/light"`, `"logger":null`, 1),
	}
	for name, line := range cases {
		t.Run(name, func(t *testing.T) {
			c := newTestCollector(t)
			if err := c.LogRecord(1, []byte(line)); !errors.Is(err, ErrIncompatible) {
				t.Fatalf("wanted incompatible, got %v", err)
			}
			if c.Diagnostics()["incompatible"] != true {
				t.Fatal("schema rejection not sticky")
			}
		})
	}
}

func TestNativeJSONFailureAndCatchupSchema(t *testing.T) {
	c := newTestCollector(t)
	line := strings.NewReplacer(
		`"level":"info"`,
		`"level":"error"`,
		`"msg":"sampled header"`,
		`"msg":"failed to sample header","err":"not available"`,
		`"EDS square width"`,
		`"square width"`,
	).
		Replace(nativeCompletion())
	if err := c.LogRecord(1, []byte(line)); err != nil {
		t.Fatal(err)
	}
	catchup := strings.NewReplacer(`"level":"info"`, `"level":"debug"`, `"type":"recent"`, `"type":"catchup"`).
		Replace(nativeCompletion())
	if err := c.LogRecord(1, []byte(catchup)); err != nil {
		t.Fatal(err)
	}
	if len(c.logs) != 2 || c.logs[0].kind != "failure" || c.logs[1].kind != "success" ||
		c.logs[1].jobType != model.JobCatchup {
		t.Fatalf("history: %+v", c.logs)
	}
}

func TestEpochLifetimeIdentityAndStickyGaps(t *testing.T) {
	c := newTestCollector(t)
	if err := c.SetNodeID("peer"); err != nil {
		t.Fatal(err)
	}
	if err := c.SetNodeID("other"); !errors.Is(err, ErrIncompatible) {
		t.Fatalf("identity drift: %v", err)
	}
	c = newTestCollector(t)
	if err := c.BeginEpoch(Epoch{Number: 2, StartedAt: baseTime}); err == nil {
		t.Fatal("overlapping epoch accepted")
	}
	c = newTestCollector(t)
	if err := c.LogRecord(1, []byte(nativeStart(rootA))); err != nil {
		t.Fatal(err)
	}
	if err := c.EndEpoch(1, baseTime.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := c.BeginEpoch(Epoch{Number: 2, StartedAt: baseTime.Add(2 * time.Second)}); err != nil {
		t.Fatal(err)
	}
	if c.Diagnostics()["roots"] != 1 {
		t.Fatal("restart erased lifetime roots")
	}
	if err := c.LogRecord(1, []byte(nativeStart(rootA))); !errors.Is(err, ErrEvidenceGap) {
		t.Fatalf("closed old boot stream reopened: %v", err)
	}
	c = newTestCollector(t)
	c.MarkGap(1, "stderr reconnect")
	if c.Diagnostics()["evidence_gap"] != true {
		t.Fatal("gap lost")
	}
	if err := c.LogRecord(1, []byte(nativeStart(rootA))); !errors.Is(err, ErrEvidenceGap) {
		t.Fatalf("sticky gap: %v", err)
	}
	c = newTestCollector(t)
	if err := c.LogRecord(99, []byte(nativeStart(rootA))); !errors.Is(err, ErrEvidenceGap) {
		t.Fatalf("unowned stream accepted: %v", err)
	}
	if err := c.EndEpoch(1, baseTime.Add(-2*time.Minute)); err == nil {
		t.Fatal("end before start")
	}
	for _, o := range []Options{
		{},
		{ChainID: "test", SampleAmount: -1},
		{ChainID: "test", SampleAmount: 16, SamplingWindow: -1},
		{ChainID: "test", SampleAmount: 16, ClockUncertainty: -1},
		{ChainID: "test", SampleAmount: 16, MaxRecords: -1},
	} {
		if _, err := New(o); !errors.Is(err, ErrIncompatible) {
			t.Fatalf("invalid options accepted: %+v, %v", o, err)
		}
	}
}

func TestRecordOverflowNeverEvictsHistory(t *testing.T) {
	c := newTestCollector(t)
	c.opts.MaxRecords = 2
	if err := c.LogRecord(1, []byte(nativeStart(rootA))); err != nil {
		t.Fatal(err)
	}
	if err := c.LogRecord(1, []byte(nativeCompletion())); !errors.Is(err, ErrEvidenceGap) {
		t.Fatalf("overflow: %v", err)
	}
	if c.Diagnostics()["roots"] != 1 || len(c.logs) != 1 {
		t.Fatal("overflow evicted history")
	}
}

func TestSealFreezesTerminalGeneration(t *testing.T) {
	c := newTestCollector(t)
	if err := c.Seal(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := c.Diagnostics()
	if err := c.LogRecord(1, []byte(nativeStart(rootA))); !errors.Is(err, ErrSealed) {
		t.Fatalf("sealed collector accepted a record: %v", err)
	}
	c.MarkGap(1, "post-seal must not mutate")
	if err := c.BeginEpoch(Epoch{Number: 2, StartedAt: baseTime}); !errors.Is(err, ErrSealed) {
		t.Fatal("sealed collector opened an epoch")
	}
	if c.Diagnostics()["evidence_gap"] != before["evidence_gap"] || c.Diagnostics()["sealed"] != true {
		t.Fatal("terminal generation changed after seal")
	}
	expired, cancel := context.WithCancel(context.Background())
	cancel()
	c = newTestCollector(t)
	if err := c.Seal(expired); !errors.Is(err, ErrEvidenceGap) {
		t.Fatalf("drain deadline sealed a clean generation: %v", err)
	}
}

func TestWitnessFromNativeLogsAndValidatedHeader(t *testing.T) {
	h := headerFixture(t, 42)
	c := collectorForHeader(t, h)
	at := h.Time().Add(time.Second).Truncate(time.Millisecond)
	logFixture(t, c, 1, h, at, "start")
	logFixture(t, c, 1, h, at.Add(200*time.Millisecond), "success")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	lookup := &localHeader{h: h}
	w, err := c.WaitWitness(ctx, WitnessRequest{Epoch: 1}, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if w.Header.Hash != h.Hash().String() || w.Header.Root != h.DAH.String() || w.Header.Height != 42 || w.Epoch != 1 ||
		w.JobType != model.JobRecent ||
		w.SampleCount != 16 ||
		w.EvidenceKind != "native_logs" {
		t.Fatalf("witness %+v", w)
	}
	if !w.StartedAt.Equal(at) || !w.CompletedAt.Equal(at.Add(201*time.Millisecond)) || w.DurationSeconds != 0.2 {
		t.Fatalf("witness timing %+v", w)
	}
	if lookup.calls == 0 {
		t.Fatal("header was not resolved through the lookup")
	}
	// A small square caps the sample count at width*width.
	roots, err := share.NewAxisRoots(edstest.RandEDS(t, 1))
	if err != nil {
		t.Fatal(err)
	}
	small := signedHeader(t, 43, roots, time.Now().Add(-time.Minute))
	c = collectorForHeader(t, small)
	at = small.Time().Add(time.Second).Truncate(time.Millisecond)
	logFixture(t, c, 1, small, at, "start")
	logFixture(t, c, 1, small, at.Add(100*time.Millisecond), "success")
	w, err = c.WaitWitness(ctx, WitnessRequest{Epoch: 1}, &localHeader{h: small})
	if err != nil || w.SampleCount != 4 {
		t.Fatalf("cap: %+v %v", w, err)
	}
}

func TestWitnessRequiresSessionStartAndCleanHistory(t *testing.T) {
	h := headerFixture(t, 42)
	other := headerFixture(t, 43)
	for _, name := range []string{"missing start", "failure after start", "start after completion", "other root only"} {
		t.Run(name, func(t *testing.T) {
			c := collectorForHeader(t, h)
			at := h.Time().Add(time.Second).Truncate(time.Millisecond)
			switch name {
			case "failure after start":
				logFixture(t, c, 1, h, at, "start")
				logFixture(t, c, 1, h, at.Add(50*time.Millisecond), "failure")
			case "start after completion":
				logFixture(t, c, 1, h, at.Add(300*time.Millisecond), "start")
			case "other root only":
				logFixture(t, c, 1, other, at, "start")
			}
			logFixture(t, c, 1, h, at.Add(200*time.Millisecond), "success")
			noWitness(t, c, h, WitnessRequest{Epoch: 1})
		})
	}
	// A retry after a failure with a fresh session start qualifies again.
	c := collectorForHeader(t, h)
	at := h.Time().Add(time.Second).Truncate(time.Millisecond)
	logFixture(t, c, 1, h, at, "start")
	logFixture(t, c, 1, h, at.Add(50*time.Millisecond), "failure")
	logFixture(t, c, 1, h, at.Add(100*time.Millisecond), "start")
	logFixture(t, c, 1, h, at.Add(200*time.Millisecond), "success")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := c.WaitWitness(ctx, WitnessRequest{Epoch: 1}, &localHeader{h: h}); err != nil {
		t.Fatal(err)
	}
}

func TestWitnessRejectsIdentityWindowAndRequestMismatch(t *testing.T) {
	h := headerFixture(t, 42)
	for _, name := range []string{
		"chain",
		"root",
		"height",
		"width",
		"empty EDS",
		"invalid header",
		"expired window",
		"future header",
		"fence",
		"min height",
		"job type",
		"wrong epoch",
		"missing node ID",
		"lookup miss",
	} {
		t.Run(name, func(t *testing.T) {
			target := h
			at := h.Time().Add(time.Second).Truncate(time.Millisecond)
			if name == "empty EDS" {
				target = headertest.NewTestSuite(t).Head()
				at = target.Time().Add(time.Second).Truncate(time.Millisecond)
			}
			c := collectorForHeader(t, target)
			req := WitnessRequest{Epoch: 1}
			lookup := &localHeader{h: target}
			switch name {
			case "chain":
				c.opts.ChainID = "wrong"
			case "expired window":
				c.opts.SamplingWindow = 1100 * time.Millisecond
			case "future header":
				at = target.Time().Add(-500 * time.Millisecond).Truncate(time.Millisecond)
			case "fence":
				req.NotBefore = at.Add(time.Second)
			case "min height":
				req.MinHeight = target.Height()
			case "job type":
				req.JobType = model.JobCatchup
			case "wrong epoch":
				req.Epoch = 2
			case "missing node ID":
				c.opts.NodeID = ""
			case "lookup miss":
				lookup = &localHeader{}
			}
			logFixture(t, c, 1, target, at, "start")
			line := logLine(target, at.Add(200*time.Millisecond), "success", model.JobRecent)
			switch name {
			case "root":
				line = []byte(strings.Replace(string(line), target.DAH.String(), rootA, 1))
			case "height":
				line = []byte(
					strings.Replace(
						string(line),
						fmt.Sprintf(`"height":%d`, target.Height()),
						fmt.Sprintf(`"height":%d`, target.Height()+1),
						1,
					),
				)
			case "width":
				line = []byte(
					strings.Replace(
						string(line),
						fmt.Sprintf(`"EDS square width":%d`, len(target.DAH.RowRoots)),
						`"EDS square width":16`,
						1,
					),
				)
			}
			if err := c.LogRecord(1, line); err != nil {
				t.Fatal(err)
			}
			if name == "invalid header" {
				raw, err := target.MarshalBinary()
				if err != nil {
					t.Fatal(err)
				}
				bad := new(header.ExtendedHeader)
				if err = bad.UnmarshalBinary(raw); err != nil {
					t.Fatal(err)
				}
				bad.RawHeader.ChainID = "bad"
				lookup = &localHeader{h: bad}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			if w, err := c.WaitWitness(ctx, req, lookup); err == nil {
				t.Fatalf("unexpected witness %+v", w)
			}
		})
	}
}

func TestWitnessRestartEpochAndFence(t *testing.T) {
	h := headerFixture(t, 42)
	after := headerFixture(t, 43)
	c := collectorForHeader(t, h)
	at := h.Time().Add(time.Second).Truncate(time.Millisecond)
	logFixture(t, c, 1, h, at, "start")
	logFixture(t, c, 1, h, at.Add(200*time.Millisecond), "success")
	if err := c.EndEpoch(1, at.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := c.BeginEpoch(Epoch{2, at.Add(2 * time.Second)}); err != nil {
		t.Fatal(err)
	}
	fence := at.Add(3 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	_, err := c.WaitWitness(ctx, WitnessRequest{Epoch: 2, NotBefore: fence}, &localHeader{h: h})
	cancel()
	if !errors.Is(err, ErrNoWitness) {
		t.Fatalf("epoch-one evidence satisfied the restart request: %v", err)
	}
	// A fresh session in epoch two after the fence qualifies, any job type.
	if err := c.LogRecord(2, logLine(after, fence.Add(time.Second), "start", "")); err != nil {
		t.Fatal(err)
	}
	if err := c.LogRecord(2, logLine(after, fence.Add(1200*time.Millisecond), "success", model.JobCatchup)); err != nil {
		t.Fatal(err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	w, err := c.WaitWitness(ctx, WitnessRequest{Epoch: 2, NotBefore: fence}, &localHeader{h: after})
	if err != nil || w.Epoch != 2 || w.JobType != model.JobCatchup || w.Header.Height != 43 {
		t.Fatalf("restart witness: %+v %v", w, err)
	}
	// Sealed generations answer immediately and keep the old witness stable.
	if err := c.EndEpoch(2, fence.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := c.Seal(context.Background()); err != nil {
		t.Fatal(err)
	}
	again, err := c.WaitWitness(
		context.Background(),
		WitnessRequest{Epoch: 2, NotBefore: fence},
		&localHeader{h: after},
	)
	if err != nil || again != w {
		t.Fatalf("terminal revalidation changed the witness: %+v %v", again, err)
	}
	if _, err = c.WaitWitness(
		context.Background(),
		WitnessRequest{Epoch: 2, NotBefore: fence, MinHeight: 43},
		&localHeader{h: after},
	); !errors.Is(
		err,
		ErrNoWitness,
	) {
		t.Fatalf("sealed collector must answer without waiting: %v", err)
	}
}

func TestGapDuringLookupCannotPass(t *testing.T) {
	h := headerFixture(t, 42)
	c := collectorForHeader(t, h)
	at := h.Time().Add(time.Second).Truncate(time.Millisecond)
	logFixture(t, c, 1, h, at, "start")
	logFixture(t, c, 1, h, at.Add(200*time.Millisecond), "success")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := c.WaitWitness(ctx, WitnessRequest{Epoch: 1}, gapLookup{c, h})
	if !errors.Is(err, ErrEvidenceGap) {
		t.Fatalf("lookup raced gap: %v", err)
	}
}

type gapLookup struct {
	c *Collector
	h *header.ExtendedHeader
}

func (g gapLookup) GetByHash(context.Context, libhead.Hash) (*header.ExtendedHeader, error) {
	g.c.MarkGap(1, "gap during local lookup")
	return g.h, nil
}

func TestEmptyBlockCompletionWithoutSession(t *testing.T) {
	empty := signedHeader(t, 42, share.EmptyEDSRoots(), time.Now().Add(-time.Minute))
	at := empty.Time().Add(time.Second).Truncate(time.Millisecond)
	t.Run("allowed for the first sample", func(t *testing.T) {
		c := collectorForHeader(t, empty)
		logFixture(t, c, 1, empty, at, "success")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		w, err := c.WaitWitness(ctx, WitnessRequest{Epoch: 1, AllowEmpty: true}, &localHeader{h: empty})
		if err != nil {
			t.Fatal(err)
		}
		if !w.Empty || w.SampleCount != 0 || w.Header.Hash != empty.Hash().String() || w.Header.Width != 2 ||
			w.JobType != model.JobRecent {
			t.Fatalf("witness %+v", w)
		}
		if !w.StartedAt.Equal(at) || !w.CompletedAt.Equal(at.Add(time.Millisecond)) {
			t.Fatalf("witness timing %+v", w)
		}
		if d := c.Diagnostics(); d["evidence_gap"] != false || d["sampled_headers"] != 1 ||
			d["sampling_sessions"] != 0 {
			t.Fatalf("ledger %v", d)
		}
	})
	t.Run("ignored, not a gap, when a session is required", func(t *testing.T) {
		c := collectorForHeader(t, empty)
		logFixture(t, c, 1, empty, at, "success")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()
		_, err := c.WaitWitness(ctx, WitnessRequest{Epoch: 1}, &localHeader{h: empty})
		if !errors.Is(err, ErrNoWitness) || errors.Is(err, ErrEvidenceGap) {
			t.Fatalf("err %v", err)
		}
	})
	t.Run("a session on an empty square is not native", func(t *testing.T) {
		c := collectorForHeader(t, empty)
		logFixture(t, c, 1, empty, at, "start")
		logFixture(t, c, 1, empty, at.Add(200*time.Millisecond), "success")
		noWitness(t, c, empty, WitnessRequest{Epoch: 1, AllowEmpty: true})
	})
	t.Run("session-less completion of a non-empty square is a gap", func(t *testing.T) {
		h := headerFixture(t, 42)
		c := collectorForHeader(t, h)
		logFixture(t, c, 1, h, h.Time().Add(time.Second).Truncate(time.Millisecond), "success")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()
		_, err := c.WaitWitness(ctx, WitnessRequest{Epoch: 1, AllowEmpty: true}, &localHeader{h: h})
		if !errors.Is(err, ErrEvidenceGap) {
			t.Fatalf("err %v", err)
		}
	})
}

// slowLookup resolves one header with the round trip of a node's RPC.
type slowLookup struct {
	h     *header.ExtendedHeader
	delay time.Duration
}

func (s slowLookup) GetByHash(_ context.Context, hash libhead.Hash) (*header.ExtendedHeader, error) {
	time.Sleep(s.delay)
	if hash.String() != s.h.Hash().String() {
		return nil, fmt.Errorf("not stored")
	}
	return s.h, nil
}

func TestWitnessIsFoundWhileRecordsKeepArriving(t *testing.T) {
	h := headerFixture(t, 42)
	c, err := New(Options{
		ChainID: h.ChainID(), NodeID: "peer", SampleAmount: 16, SamplingWindow: time.Hour, MaxRecords: 1 << 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.BeginEpoch(Epoch{1, h.Time().Add(-time.Second)}); err != nil {
		t.Fatal(err)
	}
	at := h.Time().Add(time.Second).Truncate(time.Millisecond)
	logFixture(t, c, 1, h, at, kindStart)
	logFixture(t, c, 1, h, at.Add(200*time.Millisecond), kindSuccess)

	// A node catching up after a restart logs records far faster than one
	// header lookup completes: other sessions keep starting while the witness
	// is being resolved.
	others := make([]*header.ExtendedHeader, 32)
	for i := range others {
		others[i] = headerFixture(t, uint64(100+i))
	}
	stop, flooding := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(flooding)
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			_ = c.LogRecord(1, logLine(others[i%len(others)], time.Now().UTC(), kindStart, ""))
			time.Sleep(100 * time.Microsecond)
		}
	}()
	defer func() { close(stop); <-flooding }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	started := time.Now()
	w, err := c.WaitWitness(ctx, WitnessRequest{Epoch: 1}, slowLookup{h: h, delay: 2 * time.Millisecond})
	if err != nil {
		t.Fatalf("no witness while records kept arriving: %v", err)
	}
	if w.Header.Height != 42 || time.Since(started) > time.Second {
		t.Fatalf("witness %+v after %s", w, time.Since(started))
	}
}
