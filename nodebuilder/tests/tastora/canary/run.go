package canary

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/celestiaorg/celestia-node/das"
	nativedocker "github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/docker"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/telemetry"
)

// evidenceReason is the evidence key that carries a fixed error category.
const evidenceReason = "reason"

// RunOptions bound every phase of one fresh-node observation.
type RunOptions struct {
	RunID string
	// RunTimeout bounds the whole engine run, excluding independent cleanup.
	RunTimeout time.Duration
	// BootstrapTimeout bounds RPC readiness, bootstrapper connectivity and head/tail.
	BootstrapTimeout time.Duration
	// HeaderTimeout bounds local persistence of the first HeaderCount successors.
	HeaderTimeout time.Duration
	// SamplingTimeout bounds the first sampling completion.
	SamplingTimeout time.Duration
	// RecentTimeout bounds sampling of a live head produced after startup.
	RecentTimeout time.Duration
	// RestartTimeout bounds graceful stop, restart readiness and resumed sampling.
	RestartTimeout time.Duration
	CleanupTimeout time.Duration
	PollInterval   time.Duration
	// ExitSettle bounds how long to wait for Docker to report a node exit
	// after the node's output ended: the daemon closes the attach stream when
	// the process's stdio closes and marks the container exited a moment later.
	ExitSettle time.Duration
	// ClockUncertainty tolerates skew between host clock and block timestamps.
	ClockUncertainty time.Duration
	// HeadTolerance bounds how old the network head may be at bootstrap.
	HeadTolerance time.Duration
	HeaderCount   int
}

// Reader is the part of the node's RPC the engine uses; all of it is read-only.
type Reader interface {
	HeaderReader
	Info(context.Context) (peer.AddrInfo, error)
	Peers(context.Context) ([]peer.ID, error)
	SamplingStats(context.Context) (das.SamplingStats, error)
	Close() error
}
type Session interface {
	Start(context.Context) (model.ProcessInfo, error)
	Stop(context.Context) (model.ProcessInfo, error)
	Inspect(context.Context) (model.ProcessInfo, error)
	RPC(context.Context) (Reader, error)
	Logs(context.Context) (io.ReadCloser, error)
	Store() model.StoreIdentity
	Cleanup(context.Context) error
}

func defaults(o RunOptions) RunOptions {
	if o.RunID == "" {
		o.RunID = "run-" + strings.ToLower(rand.Text())
	}
	for _, x := range []struct {
		p *time.Duration
		v time.Duration
	}{
		{&o.RunTimeout, 25 * time.Minute},
		{&o.BootstrapTimeout, 3 * time.Minute},
		{&o.HeaderTimeout, 3 * time.Minute},
		{&o.SamplingTimeout, 5 * time.Minute},
		{&o.RecentTimeout, 3 * time.Minute},
		{&o.RestartTimeout, 6 * time.Minute},
		{&o.CleanupTimeout, 30 * time.Second},
		{&o.PollInterval, time.Second},
		{&o.ExitSettle, 5 * time.Second},
		{&o.HeadTolerance, time.Minute},
		{&o.ClockUncertainty, 5 * time.Second},
	} {
		if *x.p == 0 {
			*x.p = x.v
		}
	}
	if o.HeaderCount == 0 {
		o.HeaderCount = 100
	}
	return o
}

func initialResult(p model.Profile, o RunOptions) model.Result {
	r := model.Result{SchemaVersion: "2", RunID: o.RunID, Profile: p, StartedAt: time.Now().UTC()}
	for _, name := range model.RequiredChecks() {
		r.Checks = append(r.Checks, model.Check{Name: name, Outcome: model.Inconclusive, Code: "not_run"})
	}
	return r
}

func setCheck(
	r *model.Result,
	name string,
	outcome model.Outcome,
	code string,
	start time.Time,
	evidence map[string]any,
) {
	for i := range r.Checks {
		if r.Checks[i].Name == name {
			r.Checks[i] = model.Check{
				Name:            name,
				Outcome:         outcome,
				Code:            code,
				DurationSeconds: time.Since(start).Seconds(),
				Evidence:        evidence,
			}
			return
		}
	}
}

func finish(r *model.Result) error {
	r.FinishedAt = time.Now().UTC()
	// Witness completion is the upper edge of a millisecond log timestamp; the
	// report must not end before its own evidence.
	for _, w := range r.Witnesses {
		if w.CompletedAt.After(r.FinishedAt) {
			r.FinishedAt = w.CompletedAt
		}
	}
	r.Outcome = model.Aggregate(r.Checks)
	return r.Validate()
}

func Run(ctx context.Context, p model.Profile, o RunOptions) (model.Result, error) {
	return runWithFactory(ctx, p, o, nil)
}

// sessionFactory is unexported: tests that need a controlled session build one
// on an existing Docker network and call RunSession directly.
type sessionFactory func(context.Context, model.Profile, string) (Session, error)

func runWithFactory(
	ctx context.Context,
	p model.Profile,
	o RunOptions,
	factory sessionFactory,
) (r model.Result, err error) {
	o = defaults(o)
	r = initialResult(p, o)
	if e := validateRun(p, o); e != nil {
		setCheck(
			&r,
			model.CheckFreshStore,
			model.Unsupported,
			"unsupported_profile",
			r.StartedAt,
			map[string]any{evidenceReason: e.Error()},
		)
		setCheck(&r, model.CheckCleanup, model.Pass, "no_resources", r.StartedAt, nil)
		return r, finish(&r)
	}
	ctx, cancel := context.WithTimeout(ctx, o.RunTimeout)
	defer cancel()
	c, e := telemetry.New(
		telemetry.Options{
			ChainID:          p.ChainID,
			Network:          p.Network,
			SampleAmount:     p.SampleAmount,
			SamplingWindow:   time.Duration(p.SamplingWindowSeconds) * time.Second,
			ClockUncertainty: o.ClockUncertainty,
		},
	)
	if e != nil {
		setCheck(&r, model.CheckDAS, model.Unsupported, "invalid_collector_options", r.StartedAt, nil)
		setCheck(&r, model.CheckCleanup, model.Pass, "no_resources", r.StartedAt, nil)
		return r, errors.Join(errors.New("collector creation failed"), finish(&r))
	}

	if factory == nil {
		factory = func(ctx context.Context, p model.Profile, id string) (Session, error) {
			s, err := nativedocker.New(ctx, nativedocker.Config{Profile: p, RunID: id})
			if s == nil {
				return nil, err
			}
			return AdaptSession(s), err
		}
	}
	s, e := factory(ctx, p, o.RunID)
	if e != nil || s == nil {
		setCheck(&r, model.CheckFreshStore, model.Inconclusive, "session_creation_failed", r.StartedAt, nil)
		if s == nil {
			setCheck(&r, model.CheckCleanup, model.Inconclusive, "factory_cleanup_unverified", r.StartedAt, nil)
		} else {
			clean, cc := context.WithTimeout(context.Background(), o.CleanupTimeout)
			ce := s.Cleanup(clean)
			cc()
			if ce != nil {
				setCheck(&r, model.CheckCleanup, model.Fail, "cleanup_failed", r.StartedAt, nil)
			} else {
				setCheck(&r, model.CheckCleanup, model.Pass, "removed_owned_resources", r.StartedAt, nil)
			}
		}
		return r, errors.Join(errors.New("session creation failed"), finish(&r))
	}
	started := r.StartedAt
	r, err = RunSession(ctx, p, o, s, c)
	r.StartedAt = started
	return r, errors.Join(err, r.Validate())
}

func RunSession(
	ctx context.Context,
	p model.Profile,
	o RunOptions,
	s Session,
	c *telemetry.Collector,
) (r model.Result, err error) {
	o = defaults(o)
	r = initialResult(p, o)
	terminal := &terminalVerification{}
	defer func() {
		start := time.Now()
		if s != nil {
			cleanup, cancel := context.WithTimeout(context.Background(), o.CleanupTimeout)
			e := s.Cleanup(cleanup)
			terminal.verify(cleanup, c, &r)
			cancel()
			if e != nil {
				setCheck(&r, model.CheckCleanup, model.Fail, "cleanup_failed", start, nil)
			} else {
				setCheck(&r, model.CheckCleanup, model.Pass, "removed_owned_resources", start, nil)
			}
		} else {
			setCheck(&r, model.CheckCleanup, model.Pass, "no_resources", start, nil)
		}
		err = errors.Join(err, finish(&r))
	}()
	if e := validateRun(p, o); e != nil {
		setCheck(
			&r,
			model.CheckFreshStore,
			model.Unsupported,
			"unsupported_profile",
			r.StartedAt,
			map[string]any{evidenceReason: e.Error()},
		)
		return r, err
	}
	if s == nil || c == nil {
		setCheck(&r, model.CheckFreshStore, model.Inconclusive, "missing_session_or_collector", r.StartedAt, nil)
		return r, err
	}
	store := s.Store()
	if !store.Fresh || store.Owner != o.RunID || store.VolumeID == "" {
		setCheck(&r, model.CheckFreshStore, model.Fail, "store_not_fresh_owned", r.StartedAt, nil)
		return r, err
	}
	setCheck(&r, model.CheckFreshStore, model.Pass, "fresh_owned_store", r.StartedAt, map[string]any{"store": store})
	executeSession(ctx, p, o, s, c, &r, terminal)
	return r, err
}
