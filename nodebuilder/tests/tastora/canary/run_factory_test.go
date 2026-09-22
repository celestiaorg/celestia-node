package canary

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
)

func TestRunFactoryConstructsCollectorBeforeUnstartedSession(t *testing.T) {
	s, p := newSimulation(t)
	opts := simulationOptions()
	calls := 0
	var entered time.Time
	factory := func(ctx context.Context, got model.Profile, runID string) (Session, error) {
		entered = time.Now().UTC()
		calls++
		require.Equal(t, p, got)
		require.Equal(t, "run-1", runID)
		require.Zero(t, s.epoch)
		require.NoError(t, ctx.Err())
		return s, nil
	}
	r, err := runWithFactory(context.Background(), p, opts, factory)
	require.NoError(t, err)
	require.NoError(t, r.Validate())
	require.Equal(t, model.Pass, r.Outcome, "%+v", r.Checks)
	require.True(t, s.cleaned)
	require.Equal(t, 1, calls)
	require.Len(t, r.Witnesses, 3)
	require.False(t, r.StartedAt.After(entered), "run timestamp must include resource provisioning")
}

func TestRunRejectsUnsafeOptionsBeforeFactory(t *testing.T) {
	for _, change := range []func(*RunOptions){
		func(o *RunOptions) { o.HeaderCount = 1 },
		func(o *RunOptions) { o.PollInterval = -1 },
		func(o *RunOptions) { o.RunTimeout = -1 },
		func(o *RunOptions) { o.ClockUncertainty = -1 },
		func(o *RunOptions) { o.RunID = "unsafe/id" },
	} {
		p := localProfile()
		p.Name = model.ProfileMocha
		p.Network = p2p.Mocha.String()
		p.ChainID = p2p.Mocha.String()
		o := simulationOptions()
		change(&o)
		called := false
		r, err := runWithFactory(
			context.Background(),
			p,
			o,
			func(context.Context, model.Profile, string) (Session, error) {
				called = true
				return nil, errors.New("should not run")
			},
		)
		require.False(t, called, "unsafe options reached factory")
		require.NoError(t, err)
		require.NoError(t, r.Validate())
		require.Equal(t, model.Unsupported, r.Outcome)
	}
}

func TestRunFactoryFailureCannotReturnIncompleteResult(t *testing.T) {
	p := localProfile()
	opts := simulationOptions()
	s := &failedSession{}
	factory := func(context.Context, model.Profile, string) (Session, error) {
		return s, errors.New("factory failed")
	}
	r, err := runWithFactory(context.Background(), p, opts, factory)
	require.Error(t, err)
	require.NoError(t, r.Validate())
	require.NotEqual(t, model.Pass, r.Outcome)
	require.True(t, s.cleaned)
}
