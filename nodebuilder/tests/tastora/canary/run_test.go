package canary

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/telemetry"
)

type failedSession struct {
	fresh            bool
	cleaned, started bool
	cleanupErr       error
}

func (s *failedSession) Start(context.Context) (model.ProcessInfo, error) {
	s.started = true
	return model.ProcessInfo{}, errors.New("start failed")
}

func (*failedSession) Stop(context.Context) (model.ProcessInfo, error) {
	return model.ProcessInfo{}, errors.New("unused")
}

func (*failedSession) Inspect(context.Context) (model.ProcessInfo, error) {
	return model.ProcessInfo{}, nil
}

func (*failedSession) RPC(context.Context) (Reader, error) { return nil, errors.New("unused") }

func (*failedSession) Logs(context.Context) (io.ReadCloser, error) { return nil, errors.New("unused") }

func (s *failedSession) Store() model.StoreIdentity {
	return model.StoreIdentity{Fresh: s.fresh, Owner: "run-1", VolumeID: "volume"}
}

func (s *failedSession) Cleanup(ctx context.Context) error {
	s.cleaned = ctx.Err() == nil
	return s.cleanupErr
}

func checkNamed(r model.Result, name string) model.Check {
	for _, c := range r.Checks {
		if c.Name == name {
			return c
		}
	}
	return model.Check{}
}

func collectorFor(t *testing.T, p model.Profile) *telemetry.Collector {
	c, err := telemetry.New(
		telemetry.Options{
			ChainID:        p.ChainID,
			Network:        p.Network,
			SampleAmount:   p.SampleAmount,
			SamplingWindow: time.Duration(p.SamplingWindowSeconds) * time.Second,
		},
	)
	require.NoError(t, err)
	return c
}

func TestRunSessionRejectsReusedStoreAndAlwaysCleansAfterCancel(t *testing.T) {
	p := localProfile()
	s := &failedSession{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, err := RunSession(ctx, p, RunOptions{RunID: "run-1"}, s, collectorFor(t, p))
	require.NoError(t, err)
	require.True(t, s.cleaned)
	require.False(t, s.started)
	require.Equal(t, model.Fail, checkNamed(r, "fresh_store").Outcome)
	require.NoError(t, r.Validate())
}

func TestRunReturnsExplicitUnsupportedWithoutDocker(t *testing.T) {
	p := model.Profile{Name: "arabica", Network: "arabica-11"}
	r, err := Run(context.Background(), p, RunOptions{RunID: "run-1"})
	require.NoError(t, err)
	require.Equal(t, model.Unsupported, r.Outcome)
	require.NoError(t, r.Validate())
}
