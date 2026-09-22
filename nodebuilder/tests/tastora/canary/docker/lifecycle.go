package docker

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/moby/moby/client"

	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
)

func (s *Session) inspect(ctx context.Context) (model.ProcessInfo, error) {
	if s.closed {
		return model.ProcessInfo{}, errors.New("session closed")
	}
	c, err := s.cli.ContainerInspect(ctx, s.containerID, client.ContainerInspectOptions{})
	if err != nil {
		return model.ProcessInfo{}, err
	}
	if c.Container.Config != nil && c.Container.Config.Tty {
		return model.ProcessInfo{}, errors.New("non-TTY container required")
	}
	if c.Container.ID != s.containerID || c.Container.Image != s.imageID || c.Container.Config == nil ||
		!s.owns(c.Container.Config.Labels) {
		return model.ProcessInfo{}, errors.New("container identity drift")
	}
	mounted := false
	for _, m := range c.Container.Mounts {
		if m.Type == "volume" && m.Name == s.volumeID && m.Destination == "/home/celestia" {
			mounted = true
		}
	}
	if !mounted {
		return model.ProcessInfo{}, errors.New("store identity drift")
	}
	if c.Container.State == nil {
		return model.ProcessInfo{}, errors.New("missing container state")
	}
	state := c.Container.State
	started, err := time.Parse(time.RFC3339Nano, state.StartedAt)
	if err != nil {
		return model.ProcessInfo{}, errors.New("invalid container start timestamp")
	}
	finished, err := time.Parse(time.RFC3339Nano, state.FinishedAt)
	if err != nil {
		return model.ProcessInfo{}, errors.New("invalid container finish timestamp")
	}
	return model.ProcessInfo{
		ContainerID: c.Container.ID,
		VolumeID:    s.volumeID,
		ImageID:     c.Container.Image,
		StartedAt:   started,
		FinishedAt:  finished,
		Running:     state.Running,
		ExitCode:    state.ExitCode,
		OOMKilled:   state.OOMKilled,
		ForcedKill:  state.ExitCode == 137,
	}, nil
}

func (s *Session) Inspect(ctx context.Context) (model.ProcessInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inspect(ctx)
}

func (s *Session) Start(ctx context.Context) (info model.ProcessInfo, err error) {
	s.mu.Lock()
	defer func() {
		s.mu.Unlock()
		if err != nil {
			err = errors.Join(err, s.Cleanup(context.Background()))
		}
	}()
	if s.closed {
		return info, errors.New("session closed")
	}
	if s.started && !s.graceful {
		return info, errors.New("restart requires observed graceful stop")
	}
	before, err := s.inspect(ctx)
	if err != nil {
		return info, err
	}
	if before.Running {
		return info, errors.New("container already running")
	}
	if s.logs == nil || s.logsUsed || s.logs.ended.Load() || s.logs.ctx.Err() != nil {
		return info, errors.New("start requires a fresh live log attachment")
	}
	s.logsUsed = true
	_, err = s.cli.ContainerStart(ctx, s.containerID, client.ContainerStartOptions{})
	if err != nil {
		return info, err
	}
	info, err = s.inspect(ctx)
	if err != nil {
		return info, err
	}
	if !info.Running || info.StartedAt.IsZero() {
		return info, errors.New("container did not start")
	}
	if s.started && !info.StartedAt.After(s.epoch) {
		return info, errors.New("restart did not establish a new boot epoch")
	}
	s.started = true
	s.graceful = false
	s.epoch = info.StartedAt
	return info, nil
}

// Stop sends SIGTERM and waits for the node to exit without escalating to
// SIGKILL; the caller's context bounds the wait.
func (s *Session) Stop(ctx context.Context) (model.ProcessInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return model.ProcessInfo{}, errors.New("session closed")
	}
	before, err := s.inspect(ctx)
	if err != nil {
		return before, err
	}
	if !before.Running {
		return before, errors.New("cannot prove graceful stop of a stopped container")
	}
	if !s.started || !before.StartedAt.Equal(s.epoch) {
		return before, errors.New("cannot stop an unobserved boot epoch")
	}
	s.graceful = false
	timeout := -1
	_, err = s.cli.ContainerStop(ctx, s.containerID, client.ContainerStopOptions{Signal: "SIGTERM", Timeout: &timeout})
	if err != nil {
		return model.ProcessInfo{}, err
	}
	info, err := s.inspect(ctx)
	if err != nil {
		return info, err
	}
	if info.Running || info.ExitCode != 0 || info.OOMKilled || info.ForcedKill || info.FinishedAt.IsZero() ||
		info.FinishedAt.Before(info.StartedAt) {
		return info, errors.New("container stop was not proven graceful")
	}
	s.graceful = true
	return info, nil
}

// Logs attaches to the container before Start, so the stream carries the output
// of the next boot; an attach sees live output only, not earlier logs. The
// caller binds the StartedAt returned by Start before reading the stream and
// treats an unexpected EOF as a gap.
func (s *Session) Logs(ctx context.Context) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	info, err := s.inspect(ctx)
	if err != nil {
		return nil, err
	}
	if info.Running {
		return nil, errors.New("logs must attach before start")
	}
	if s.logs != nil && !s.logs.ended.Load() {
		return nil, errors.New("previous log attachment must be closed")
	}
	if s.logs != nil {
		s.logs.Close()
	}
	attached, err := s.cli.ContainerAttach(
		ctx,
		s.containerID,
		client.ContainerAttachOptions{Stream: true, Logs: false, Stdout: true, Stderr: true, Stdin: false},
	)
	if err != nil {
		return nil, errors.New("native lifetime log attach failed")
	}
	stream := &attachStream{reader: attached.Reader, close: attached.Close, ctx: ctx}
	// Callback only closes the transport; it cannot call into Session's mutex.
	stream.stop = context.AfterFunc(ctx, func() { stream.ended.Store(true); stream.closeOnce.Do(stream.close) })
	s.logs = stream
	s.logsUsed = false
	return stream, nil
}

type attachStream struct {
	reader    io.Reader
	close     func()
	stop      func() bool
	ctx       context.Context
	closeOnce sync.Once
	ended     atomic.Bool
}

func (s *attachStream) Read(p []byte) (int, error) {
	n, err := s.reader.Read(p)
	if err != nil {
		s.ended.Store(true)
	}
	return n, err
}

func (s *attachStream) Close() error {
	s.ended.Store(true)
	s.stop()
	s.closeOnce.Do(s.close)
	return nil
}
