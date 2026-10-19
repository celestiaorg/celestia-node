package canary

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
)

type cleanupGapSession struct{ *simSession }

func (s cleanupGapSession) Cleanup(ctx context.Context) error {
	s.c.MarkGap(1, "synthetic pending ingress loss during cleanup")
	return s.simSession.Cleanup(ctx)
}

func TestRunSessionRejectsEvidenceChangeDuringCleanup(t *testing.T) {
	sim, p := newSimulation(t)
	r, err := RunSession(context.Background(), p, simulationOptions(), cleanupGapSession{sim}, sim.c)
	require.NoError(t, err)
	require.True(t, sim.cleaned)
	require.NotEqual(t, model.Pass, r.Outcome, "cleanup-time gap must invalidate both selected witnesses")
	require.NotEqual(t, model.Pass, checkNamed(r, "das").Outcome)
	require.NotEqual(t, model.Pass, checkNamed(r, "restart").Outcome)
}

// slowCleanupSession removes everything but uses up its whole cleanup budget,
// as a slow Docker daemon does, and reports the deadline.
type slowCleanupSession struct{ *simSession }

func (s slowCleanupSession) Cleanup(ctx context.Context) error {
	if err := s.simSession.Cleanup(ctx); err != nil {
		return err
	}
	<-ctx.Done()
	return ctx.Err()
}

func TestSlowCleanupKeepsEarlierResults(t *testing.T) {
	sim, p := newSimulation(t)
	r, err := RunSession(context.Background(), p, simulationOptions(), slowCleanupSession{sim}, sim.c)
	require.NoError(t, err)
	require.Equal(t, model.Fail, checkNamed(r, model.CheckCleanup).Outcome)
	require.Equal(t, model.Pass, checkNamed(r, model.CheckDAS).Outcome, checkNamed(r, model.CheckDAS).Code)
	require.Equal(t, model.Pass, checkNamed(r, model.CheckRestart).Outcome, checkNamed(r, model.CheckRestart).Code)
}
