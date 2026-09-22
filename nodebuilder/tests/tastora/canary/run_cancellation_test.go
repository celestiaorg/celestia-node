package canary

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/stretchr/testify/require"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/telemetry"
)

// Cancel only after the real observation loop has read the nonempty header.
// No sleep or scheduling assumption chooses the cancellation boundary.
type samplingCancelReader struct {
	*simSession
	cancel       context.CancelFunc
	seenEligible bool
}

func (r *samplingCancelReader) NetworkHead(ctx context.Context) (*header.ExtendedHeader, error) {
	h, err := r.simSession.NetworkHead(ctx)
	if err == nil && h.Height() == r.hs[100].Height() {
		r.seenEligible = true
	}
	return h, err
}

func (r *samplingCancelReader) SamplingStats(ctx context.Context) (das.SamplingStats, error) {
	if r.seenEligible {
		r.cancel()
	}
	return r.simSession.SamplingStats(ctx)
}

func samplingCancellationFixture(t *testing.T, withLogs bool) (*simSession, model.Profile, *logCapture) {
	t.Helper()
	s, p := newSimulation(t)
	require.NoError(t, s.c.BeginEpoch(telemetry.Epoch{Number: 1, StartedAt: time.Now().UTC()}))
	pi, err := s.Info(context.Background())
	require.NoError(t, err)
	require.NoError(t, s.c.SetNodeID(pi.ID.String()))
	if !withLogs {
		return s, p, nil
	}
	stream, writer := io.Pipe()
	cap := captureLogs(context.Background(), stream, s.c, 1, nil, 0)
	t.Cleanup(func() {
		cap.close()
		writer.Close()
	})
	written := make(chan error, 1)
	go func() {
		_, err := (muxFixture{writer, stdcopy.Stderr}).Write(
			[]byte("{\"logger\":\"p2p\",\"level\":\"info\",\"msg\":\"synthetic fixture alive\"}\n"),
		)
		written <- err
	}()
	select {
	case err := <-written:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("unrelated log was not consumed")
	}
	require.Eventually(t, func() bool { return cap.bytes.Load() > 0 }, time.Second, time.Millisecond)
	return s, p, cap
}

func TestSamplingCancellationRemainsInconclusive(t *testing.T) {
	s, p, cap := samplingCancellationFixture(t, true)
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, deadlineCancel := context.WithTimeout(parent, time.Second)
	defer deadlineCancel()
	reader := &samplingCancelReader{simSession: s, cancel: cancel}
	w, out, code, evidence := waitSampling(
		ctx,
		reader,
		s.c,
		telemetry.WitnessRequest{Epoch: 1},
		p,
		simulationOptions(),
		cap,
		&witnessHeaders{},
		phaseFirstSample,
	)
	require.ErrorIs(t, ctx.Err(), context.Canceled, "must abort before the observation deadline")
	require.Equal(t, true, evidence["newer_nonempty_head_observed"])
	require.Equal(t, model.Witness{}, w, "no synthetic success evidence was supplied")
	require.Equal(t, model.Inconclusive, out, "operator abort is not failed autonomous DAS: %s", code)
	require.Equal(t, "canceled", code)
}

func TestSamplingDeadlineAndEvidencePrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, problem, code string
		deadline, logs      bool
		out                 model.Outcome
	}{
		{"deadline_with_eligible_header_and_logs", "", "sampling_timeout", true, true, model.Fail},
		{"deadline_without_logs", "", "evidence_gap", true, false, model.Inconclusive},
		{"cancel_without_logs", "", "canceled", false, false, model.Inconclusive},
		{"cancel_with_gap", "gap", "evidence_gap", false, true, model.Inconclusive},
		{"cancel_with_incompatible", "incompatible", "incompatible_evidence", false, true, model.Unsupported},
		{"deadline_with_gap", "gap", "evidence_gap", true, true, model.Inconclusive},
		{"deadline_with_incompatible", "incompatible", "incompatible_evidence", true, true, model.Unsupported},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, p, cap := samplingCancellationFixture(t, tc.logs)
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx, deadlineCancel := context.WithTimeout(parent, 300*time.Millisecond)
			defer deadlineCancel()
			reader := &samplingCancelReader{simSession: s, cancel: func() {
				switch tc.problem {
				case "gap":
					s.c.MarkGap(1, "synthetic missing evidence")
				case "incompatible":
					require.ErrorIs(t, s.c.SetNodeID("synthetic-conflicting-id"), telemetry.ErrIncompatible)
				}
				if tc.deadline {
					<-ctx.Done()
				} else {
					cancel()
				}
			}}
			w, out, code, evidence := waitSampling(
				ctx,
				reader,
				s.c,
				telemetry.WitnessRequest{Epoch: 1},
				p,
				simulationOptions(),
				cap,
				&witnessHeaders{},
				phaseFirstSample,
			)
			if tc.deadline {
				require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
				require.NoError(t, parent.Err(), "only the observation phase expired")
			} else {
				require.ErrorIs(t, ctx.Err(), context.Canceled)
			}
			require.Equal(t, true, evidence["newer_nonempty_head_observed"])
			require.Equal(t, model.Witness{}, w)
			require.Equal(t, tc.out, out)
			require.Equal(t, tc.code, code)
		})
	}
}

// Only the interruption is simulated; RunSession, header validation, log
// capture, the collector, restart and cleanup run the production code.
type cancellationSession struct {
	*simSession
	cancel                  context.CancelFunc
	abortAt, reached        string
	inspects, starts, stops int
	problem, interrupted    error
}

func (s *cancellationSession) interrupt(ctx context.Context, at string) error {
	if s.abortAt != at {
		return nil
	}
	s.reached = at
	if s.cancel == nil {
		<-ctx.Done()
	} else {
		s.cancel()
	}
	s.interrupted = ctx.Err()
	return fmt.Errorf("synthetic interrupted %s: %w", at, errors.Join(s.interrupted, s.problem))
}

func (s *cancellationSession) Inspect(ctx context.Context) (model.ProcessInfo, error) {
	s.inspects++
	at := "initial_inspect"
	if s.inspects > 1 {
		at = "stop_inspect"
	}
	if err := s.interrupt(ctx, at); err != nil {
		return model.ProcessInfo{}, err
	}
	return s.simSession.Inspect(ctx)
}

func runLifecycleCancellation(t *testing.T, at, check string, witnesses int) {
	t.Helper()
	sim, p := newSimulation(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &cancellationSession{simSession: sim, cancel: cancel, abortAt: at}
	r, err := RunSession(ctx, p, simulationOptions(), s, s.c)
	require.NoError(t, err)
	require.Equal(t, at, s.reached, "must reach the intended boundary: %+v", r.Checks)
	require.ErrorIs(t, ctx.Err(), context.Canceled)
	got := checkNamed(r, check)
	require.Equal(t, model.Inconclusive, got.Outcome, "operator abort is not a lifecycle fault: %+v", got)
	require.Equal(t, "canceled", got.Code)
	require.Equal(t, model.Inconclusive, r.Outcome)
	require.Len(t, r.Witnesses, witnesses)
	require.Equal(t, model.Pass, checkNamed(r, "cleanup").Outcome)
	require.True(t, s.cleaned, "cleanup must use a cancellation-independent context")
	require.NoError(t, r.Validate())
}

func (s *cancellationSession) Stop(ctx context.Context) (model.ProcessInfo, error) {
	s.stops++
	at := "restart_stop"
	if s.stops > 1 {
		at = "final_stop"
	}
	if err := s.interrupt(ctx, at); err != nil {
		return model.ProcessInfo{}, err
	}
	return s.simSession.Stop(ctx)
}

func TestRunRestartStopCancellationRemainsInconclusive(t *testing.T) {
	for _, at := range []string{"restart_stop", "stop_inspect"} {
		t.Run(at, func(t *testing.T) {
			runLifecycleCancellation(t, at, "restart", 2)
		})
	}
}

func (s *cancellationSession) Start(ctx context.Context) (model.ProcessInfo, error) {
	s.starts++
	at := "initial_start"
	if s.starts > 1 {
		at = "restart_start"
	}
	if err := s.interrupt(ctx, at); err != nil {
		return model.ProcessInfo{}, err
	}
	return s.simSession.Start(ctx)
}

func TestRunRestartStartCancellationRemainsInconclusive(t *testing.T) {
	runLifecycleCancellation(t, "restart_start", "restart", 2)
}

func TestRunInitialStartCancellationRemainsInconclusive(t *testing.T) {
	runLifecycleCancellation(t, "initial_start", "bootstrap", 0)
}

type cancellationReader struct {
	Reader
	session *cancellationSession
}

func (s *cancellationSession) RPC(ctx context.Context) (Reader, error) {
	r, err := s.simSession.RPC(ctx)
	return &cancellationReader{Reader: r, session: s}, err
}

func (r *cancellationReader) GetByHash(ctx context.Context, hash libhead.Hash) (*header.ExtendedHeader, error) {
	if r.session.starts == 2 {
		if err := r.session.interrupt(ctx, "retained_header"); err != nil {
			return nil, err
		}
	}
	return r.Reader.GetByHash(ctx, hash)
}

func TestRunRetainedHeaderCancellationRemainsInconclusive(t *testing.T) {
	runLifecycleCancellation(t, "retained_header", "restart", 2)
}

func TestRunFinalStopCancellationRemainsInconclusive(t *testing.T) {
	runLifecycleCancellation(t, "final_stop", "restart", 2)
}

func TestRunLifecycleDeadlineAndEvidencePrecedence(t *testing.T) {
	for _, boundary := range []struct{ at, check string }{
		{"initial_inspect", "fresh_store"},
		{"initial_start", "bootstrap"},
		{"restart_stop", "restart"},
		{"stop_inspect", "restart"},
		{"restart_start", "restart"},
		{"retained_header", "restart"},
		{"final_stop", "restart"},
	} {
		for _, tc := range []struct {
			name, code string
			problem    error
			out        model.Outcome
		}{
			{"phase_deadline", "phase_not_completed", nil, model.Fail},
			{"cancel_with_gap", "evidence_gap", telemetry.ErrEvidenceGap, model.Inconclusive},
			{
				"cancel_with_incompatible",
				"incompatible_evidence",
				errors.Join(telemetry.ErrNoWitness, telemetry.ErrEvidenceGap, telemetry.ErrIncompatible),
				model.Unsupported,
			},
			{"operation_error", "phase_not_completed", errors.New("synthetic operation failure"), model.Fail},
		} {
			t.Run(boundary.at+"/"+tc.name, func(t *testing.T) {
				sim, p := newSimulation(t)
				parent, cancel := context.WithCancel(context.Background())
				defer cancel()
				s := &cancellationSession{simSession: sim, abortAt: boundary.at, problem: tc.problem}
				if tc.name == "operation_error" {
					s.cancel = func() {}
				} else if tc.problem != nil {
					s.cancel = cancel
				}
				o := simulationOptions()
				o.BootstrapTimeout = 400 * time.Millisecond
				o.RestartTimeout = 400 * time.Millisecond
				r, err := RunSession(parent, p, o, s, s.c)
				require.NoError(t, err)
				require.Equal(t, boundary.at, s.reached, "must reach intended boundary: %+v", r.Checks)
				switch tc.name {
				case "phase_deadline":
					require.ErrorIs(t, s.interrupted, context.DeadlineExceeded)
					require.NoError(t, parent.Err(), "operator did not abort")
				case "operation_error":
					require.NoError(t, s.interrupted)
					require.NoError(t, parent.Err())
				default:
					require.ErrorIs(t, s.interrupted, context.Canceled)
				}
				got := checkNamed(r, boundary.check)
				require.Equal(t, tc.out, got.Outcome)
				require.Equal(t, tc.code, got.Code)
				require.Equal(t, tc.out, r.Outcome)
				require.True(t, s.cleaned)
				require.Equal(t, model.Pass, checkNamed(r, "cleanup").Outcome)
				require.NoError(t, r.Validate())
			})
		}
	}
}

type canceledInvalidInspectSession struct {
	*simSession
	cancel context.CancelFunc
}

func (s *canceledInvalidInspectSession) Inspect(ctx context.Context) (model.ProcessInfo, error) {
	info, err := s.simSession.Inspect(ctx)
	info.Running = true // An actual returned observation, not an interrupted call.
	s.cancel()
	return info, err
}

func TestRunCancellationDoesNotEraseInvalidProcessEvidence(t *testing.T) {
	sim, p := newSimulation(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &canceledInvalidInspectSession{simSession: sim, cancel: cancel}
	r, err := RunSession(ctx, p, simulationOptions(), s, s.c)
	require.NoError(t, err)
	require.ErrorIs(t, ctx.Err(), context.Canceled)
	require.Equal(t, model.Fail, checkNamed(r, "fresh_store").Outcome)
	require.Equal(t, "process_not_fresh", checkNamed(r, "fresh_store").Code)
	require.Equal(t, model.Fail, r.Outcome)
	require.True(t, s.cleaned)
}

func TestRunInitialInspectCancellationRemainsInconclusive(t *testing.T) {
	runLifecycleCancellation(t, "initial_inspect", "fresh_store", 0)
}
