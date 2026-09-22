package docker

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStartRequiresFreshLivePreattach(t *testing.T) {
	const (
		missing        = "missing"
		closed         = "closed"
		failed         = "failed"
		canceled       = "canceled"
		restartMissing = "restart missing"
	)
	for _, kind := range []string{missing, closed, failed, canceled, restartMissing} {
		t.Run(kind, func(t *testing.T) {
			f := &mobyFixture{}
			fixture(t, f)
			s, err := New(context.Background(), config())
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = s.Cleanup(context.Background()) }()
			switch kind {
			case closed:
				l, e := s.Logs(context.Background())
				if e != nil {
					t.Fatal(e)
				}
				l.Close()
			case canceled:
				ctx, cancel := context.WithCancel(context.Background())
				l, e := s.Logs(ctx)
				if e != nil {
					t.Fatal(e)
				}
				defer l.Close()
				cancel()
			case failed:
				f.attachFail = true
				if _, e := s.Logs(context.Background()); e == nil {
					t.Fatal("failed preattach accepted")
				}
			case restartMissing:
				l, e := s.Logs(context.Background())
				if e != nil {
					t.Fatal(e)
				}
				defer l.Close()
				if _, e = s.Start(context.Background()); e != nil {
					t.Fatal(e)
				}
				if _, e = s.Stop(context.Background()); e != nil {
					t.Fatal(e)
				}
				_, _ = io.Copy(io.Discard, l)
				l.Close()
			}
			before := f.starts
			if _, err = s.Start(context.Background()); err == nil {
				t.Fatal("start without fresh live preattach accepted")
			}
			if f.starts != before {
				t.Fatal("Docker start reached without lifetime logs")
			}
			for _, r := range f.requests {
				if strings.HasSuffix(r, "/logs") {
					t.Fatal("replay fallback")
				}
			}
		})
	}
}

func TestPreattachContextCancellationClosesRead(t *testing.T) {
	f := &mobyFixture{}
	fixture(t, f)
	s, err := New(context.Background(), config())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Cleanup(context.Background()) }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l, err := s.Logs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	done := make(chan error, 1)
	go func() { _, err := io.Copy(io.Discard, l); done <- err }()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("canceled attach leaked reader")
	}
}

func TestPreattachRejectsTTYDrift(t *testing.T) {
	f := &mobyFixture{}
	fixture(t, f)
	s, err := New(context.Background(), config())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Cleanup(context.Background()) }()
	f.container["Config"].(map[string]any)["Tty"] = true
	l, err := s.Logs(context.Background())
	if l != nil {
		l.Close()
	}
	if err == nil || f.attachCalls != 0 {
		t.Fatal("TTY stream would be treated as Docker mux")
	}
}

func TestPreattachUnexpectedCloseHasNoFallback(t *testing.T) {
	f := &mobyFixture{}
	fixture(t, f)
	s, err := New(context.Background(), config())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Cleanup(context.Background()) }()
	l, err := s.Logs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	f.mu.Lock()
	f.attachConn.Close()
	f.attachConn = nil
	f.mu.Unlock()
	b := make([]byte, 1)
	if _, err = l.Read(b); !errors.Is(err, io.EOF) {
		t.Fatalf("unexpected-close not visible: %v", err)
	}
	if _, err = s.Start(context.Background()); err == nil {
		t.Fatal("start after observed prebirth stream loss accepted")
	}
	for _, r := range f.requests {
		if strings.HasSuffix(r, "/logs") {
			t.Fatal("stream loss concealed by fallback")
		}
	}
}

func TestCleanupClosesAttachAndConcurrentCloseIsSafe(t *testing.T) {
	f := &mobyFixture{}
	fixture(t, f)
	s, err := New(context.Background(), config())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Cleanup(context.Background()) }()
	l, err := s.Logs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, l); close(done) }()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Go(func() { l.Close() })
	}
	wg.Go(func() { _ = s.Cleanup(context.Background()) })
	wg.Wait()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cleanup left attach reader blocked")
	}
}
