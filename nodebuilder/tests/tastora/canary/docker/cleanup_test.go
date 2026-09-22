package docker

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moby/moby/client"
)

func TestCleanupCancellationIndependent(t *testing.T) {
	for _, mode := range []string{"before-no-deadline", "before-future-deadline", "during-request"} {
		t.Run(mode, func(t *testing.T) {
			f := &mobyFixture{}
			var cleaning atomic.Bool
			var ctx context.Context
			var cancel context.CancelFunc
			if mode == "before-no-deadline" {
				ctx, cancel = context.WithCancel(context.Background())
			} else {
				ctx, cancel = context.WithTimeout(context.Background(), time.Second)
			}
			defer cancel()
			reviewFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if cleaning.Load() && mode == "during-request" {
					cancel()
				}
				f.ServeHTTP(w, r)
			}))
			s, err := New(context.Background(), config())
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = s.Cleanup(context.Background()) }()
			if mode != "during-request" {
				cancel()
			}
			cleaning.Store(true)
			if err := s.Cleanup(ctx); err != nil {
				t.Fatal("caller cancellation poisoned independent cleanup:", err)
			}
			if ctx.Err() != context.Canceled {
				t.Fatal("test did not cancel incoming context")
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.container != nil || f.volume != nil || f.network != nil {
				t.Fatal("canceled caller leaked owned resources")
			}
		})
	}
}

func TestCleanupExpiredDeadlineDoesNotRestartBudget(t *testing.T) {
	f := &mobyFixture{}
	fixture(t, f)
	s, err := New(context.Background(), config())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Cleanup(context.Background()) }()
	f.mu.Lock()
	before := len(f.requests)
	f.mu.Unlock()
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if err := s.Cleanup(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expired deadline gained fresh budget: %v", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.requests) != before || f.container == nil || f.volume == nil || f.network == nil {
		t.Fatal("expired cleanup reached Docker or mutated resources")
	}
}

func TestCleanupGateWaitAndRequestsShareDeadline(t *testing.T) {
	f := &mobyFixture{}
	var cleaning atomic.Bool
	var inspections atomic.Int64
	entered := make(chan struct{})
	reviewFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cleaning.Load() && r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/containers/") {
			_, _ = io.Copy(io.Discard, r.Body)
			delay := 400 * time.Millisecond
			if inspections.Add(1) == 1 {
				close(entered)
				delay = 150 * time.Millisecond
			}
			select {
			case <-r.Context().Done():
				return
			case <-time.After(delay):
			}
		}
		f.ServeHTTP(w, r)
	}))
	s, err := New(context.Background(), config())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleaning.Store(false)
		if err := s.Cleanup(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	cleaning.Store(true)
	done := make(chan error, 1)
	go func() { _, err := s.Inspect(context.Background()); done <- err }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("Inspect did not own gate")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	// Cancellation must be ignored even while cleanup is waiting on the gate.
	cancel()
	started := time.Now()
	err = s.Cleanup(ctx)
	elapsed := time.Since(started)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("want original independent deadline: %v", err)
	}
	if elapsed < 200*time.Millisecond || elapsed > 350*time.Millisecond {
		t.Errorf("wait/request budget restarted or cancellation inherited: %s", elapsed)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if inspections.Load() != 2 {
		t.Fatalf("did not exercise wait then cleanup HTTP request: %d", inspections.Load())
	}
}

func TestCleanupRacesWithNativeSessionOperations(t *testing.T) {
	f := &mobyFixture{rpcAddress: "127.0.0.1:12345"}
	fixture(t, f)
	s, err := New(context.Background(), config())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Cleanup(context.Background()) }()
	if _, err = fixtureStart(t, s); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	ready := make(chan struct{})
	operations := []func(){
		func() { _, _ = s.Start(ctx) },
		func() { _, _ = s.Stop(ctx) },
		func() { _, _ = s.Inspect(ctx) },
		func() { s.Store() },
		func() {
			c, _ := s.RPC(ctx)
			if c != nil {
				c.Close()
			}
		},
		func() {
			l, _ := s.Logs(ctx)
			if l != nil {
				l.Close()
			}
		},
		func() { _ = s.Cleanup(ctx) },
		func() { _ = s.Cleanup(ctx) },
	}
	for _, op := range operations {
		wg.Go(func() { <-ready; op() })
	}
	close(ready)
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("native operations/cleanup deadlocked")
	}
	if err := s.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.container != nil || f.volume != nil || f.network != nil {
		t.Fatal("concurrent cleanup leaked owned resources")
	}
}

func TestRacedCollisionPreservesForeignAndInUseResources(t *testing.T) {
	const (
		foreignVolume    = "foreign-volume"
		foreignNetwork   = "foreign-network"
		unknownContainer = "unknown-container"
		missingConfig    = "missing-config"
		inUse            = "in-use"
		borrowedNetwork  = "borrowed-network"
	)
	for _, mode := range []string{
		foreignVolume,
		foreignNetwork,
		unknownContainer,
		missingConfig,
		inUse,
		borrowedNetwork,
	} {
		t.Run(mode, func(t *testing.T) {
			f := &mobyFixture{}
			cfg := config()
			if mode == borrowedNetwork {
				cfg.ExistingNetworkID = borrowedNetworkID
				f.network = map[string]any{
					"Id":     borrowedNetworkID,
					"Driver": "bridge",
					"Labels": map[string]string{runLabel: "other"},
					"IPAM":   map[string]any{"Config": []any{map[string]string{"Gateway": "127.0.0.1"}}},
				}
			}
			raced := false // All accesses below are inside fixture mutex.
			reviewFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				f.mu.Lock()
				p := strings.TrimPrefix(r.URL.Path, "/v1.55")
				if p == "/containers/create" {
					raced = true
					f.container = foreignContainer()
					if mode == missingConfig {
						delete(f.container, "Config")
					}
					if mode == foreignVolume {
						f.volume["Labels"] = map[string]string{runLabel: "other", ownerLabel: "other"}
					}
					if mode == foreignNetwork {
						f.network["Labels"] = map[string]string{runLabel: "other", ownerLabel: "other"}
					}
				}
				reject := raced &&
					((mode == unknownContainer && r.Method == http.MethodGet && strings.HasPrefix(
						p,
						"/containers/",
					)) || (mode == inUse && r.Method == http.MethodDelete))
				if reject {
					f.requests = append(f.requests, r.Method+" "+p)
					f.mu.Unlock()
					_, _ = io.Copy(io.Discard, r.Body)
					if r.URL.Query().Get("force") == "1" || r.URL.Query().Get("force") == "true" {
						t.Error("forced removal bypassed Docker in-use protection")
					}
					w.Header().Set("Content-Type", "application/json")
					if mode == inUse {
						w.WriteHeader(http.StatusConflict)
					} else {
						w.WriteHeader(http.StatusInternalServerError)
					}
					reply(w, map[string]string{"message": "resource unavailable"})
					return
				}
				f.mu.Unlock()
				f.ServeHTTP(w, r)
			}))
			s, err := New(context.Background(), cfg)
			if s != nil || err == nil {
				t.Fatalf("raced failure accepted: %v", err)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.container == nil {
				t.Fatal("foreign container removed")
			}
			wantVolume := mode == foreignVolume || mode == unknownContainer || mode == missingConfig ||
				mode == inUse
			wantNetwork := mode == foreignNetwork || mode == unknownContainer || mode == missingConfig ||
				mode == inUse ||
				mode == borrowedNetwork
			if (f.volume != nil) != wantVolume || (f.network != nil) != wantNetwork {
				t.Fatalf(
					"ownership/uncertainty/in-use protection violated: volume remains=%t network remains=%t",
					f.volume != nil,
					f.network != nil,
				)
			}
			for _, r := range f.requests {
				if strings.HasPrefix(r, "DELETE /containers/") {
					t.Errorf("foreign container removal attempted: %s", r)
				}
			}
		})
	}
}

func TestCleanupRejectsUnresolvedExactContainerIdentity(t *testing.T) {
	for _, id := range []string{"", "different-container"} {
		t.Run("id-"+id, func(t *testing.T) {
			f := &mobyFixture{}
			fixture(t, f)
			s, err := New(context.Background(), config())
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				f.mu.Lock()
				if f.container != nil {
					f.container["Id"] = "container-owned"
				}
				f.mu.Unlock()
				_ = s.Cleanup(context.Background())
			}()
			f.mu.Lock()
			f.container["Id"] = id
			f.mu.Unlock()
			if err := s.Cleanup(context.Background()); err == nil {
				t.Error("unknown exact container identity accepted for cleanup")
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.container == nil || f.volume == nil || f.network == nil {
				t.Error("unresolved container identity allowed resource deletion")
			}
		})
	}
}

func TestCanceledCreateReconcilesByInspectedExactContainerID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &mobyFixture{cancelCreate: cancel}
	fixture(t, f)
	s, err := New(ctx, config())
	if s != nil || err == nil {
		t.Fatal("ambiguous create accepted")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var deletes []string
	for _, r := range f.requests {
		if strings.HasPrefix(r, "DELETE ") {
			deletes = append(deletes, r)
		}
	}
	if !reflect.DeepEqual(
		deletes,
		[]string{"DELETE /containers/container-owned", "DELETE /volumes/cfn-test-data", "DELETE /networks/net-owned"},
	) {
		t.Fatalf("cleanup must bind inspected ID before deletion, not re-resolve reusable name: %v", deletes)
	}
	if f.container != nil || f.volume != nil || f.network != nil {
		t.Fatal("ambiguous owned resources remain")
	}
}

type cleanupDeadlineTransport struct {
	base  http.RoundTripper
	check func(context.Context)
}

func (r cleanupDeadlineTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.check(req.Context())
	return r.base.RoundTrip(req)
}

// Observe the real SDK request context while forwarding every request to the
// live HTTP fixture. This checks long/default budgets without a 45-second sleep.
func TestCleanupDefaultAndLongBudgetReachActualRequests(t *testing.T) {
	for _, mode := range []string{"default-30s", "configured-45s"} {
		t.Run(mode, func(t *testing.T) {
			f := &mobyFixture{}
			fixture(t, f)
			s, err := New(context.Background(), config())
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = s.Cleanup(context.Background()) }()
			original := s.cli
			defer original.Close()
			base := http.DefaultTransport.(*http.Transport).Clone()
			defer base.CloseIdleConnections()
			var observed []time.Time
			forwarded := cleanupDeadlineTransport{base: base, check: func(ctx context.Context) {
				deadline, ok := ctx.Deadline()
				if !ok || ctx.Done() == nil || ctx.Err() != nil {
					t.Error("missing or canceled request cleanup budget")
				}
				observed = append(observed, deadline)
			}}
			s.cli, err = client.New(client.FromEnv, client.WithHTTPClient(&http.Client{Transport: forwarded}))
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			exact := time.Now().Add(45 * time.Second)
			if mode == "configured-45s" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(ctx, exact)
				cancel()
			}
			started := time.Now()
			if err := s.Cleanup(ctx); err != nil {
				t.Fatal(err)
			}
			if len(observed) == 0 {
				t.Fatal("observer bypassed: no actual SDK requests forwarded")
			}
			for _, deadline := range observed {
				if !deadline.Equal(observed[0]) {
					t.Error("cleanup request budget reset between operations")
				}
				if mode == "configured-45s" && !deadline.Equal(exact) {
					t.Errorf("configured absolute budget capped/discarded: want=%s got=%s", exact, deadline)
				}
				if mode == "default-30s" &&
					(deadline.Before(started.Add(30*time.Second)) ||
						deadline.After(started.Add(30*time.Second+100*time.Millisecond))) {
					t.Errorf("wrong no-deadline default: %s", deadline.Sub(started))
				}
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.container != nil || f.volume != nil || f.network != nil {
				t.Fatal("observed cleanup did not remove actual fixture resources")
			}
		})
	}
}

func foreignContainer() map[string]any {
	return map[string]any{
		"Id":     "container-foreign",
		"Config": map[string]any{"Labels": map[string]string{runLabel: "other", ownerLabel: "foreign-owner"}},
	}
}

func reviewFixture(t *testing.T, handler http.Handler) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	t.Setenv("DOCKER_HOST", "tcp://"+strings.TrimPrefix(srv.URL, "http://"))
	t.Setenv("DOCKER_API_VERSION", "1.55")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")
}

func TestForeignContainerRacedCollisionCleansOnlyOwnedResources(t *testing.T) {
	f := &mobyFixture{}
	reviewFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/containers/create") {
			f.mu.Lock()
			f.container = foreignContainer()
			f.mu.Unlock()
		}
		f.ServeHTTP(w, r)
	}))
	s, err := New(context.Background(), config())
	if s != nil || err == nil {
		t.Fatalf("raced collision accepted: session=%v err=%v", s, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if !reflect.DeepEqual(f.container, foreignContainer()) {
		t.Fatal("foreign container changed")
	}
	if f.volume != nil || f.network != nil {
		t.Error("raced foreign collision leaked owned volume/network")
	}
	deletes := []string{}
	for _, request := range f.requests {
		if strings.HasPrefix(request, "DELETE ") {
			deletes = append(deletes, request)
		}
	}
	if !reflect.DeepEqual(deletes, []string{"DELETE /volumes/cfn-test-data", "DELETE /networks/net-owned"}) {
		t.Errorf("wrong exact-owned reconciliation: %v", deletes)
	}
	if f.create == nil {
		t.Fatal("test failed to reach raced create")
	}
}

func TestCleanupPreservesConfiguredAbsoluteDeadline(t *testing.T) {
	f := &mobyFixture{}
	var block atomic.Bool
	reviewFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if block.Load() && r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/containers/") {
			_, _ = io.Copy(io.Discard, r.Body)
			select {
			case <-r.Context().Done():
				return
			case <-time.After(400 * time.Millisecond):
			}
		}
		f.ServeHTTP(w, r)
	}))
	s, err := New(context.Background(), config())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		block.Store(false)
		if err := s.Cleanup(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(80*time.Millisecond))
	defer cancel()
	block.Store(true)
	started := time.Now()
	err = s.Cleanup(ctx)
	elapsed := time.Since(started)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("configured cleanup deadline discarded: %v", err)
	}
	if elapsed > 300*time.Millisecond {
		t.Errorf("cleanup exceeded configured budget: %s", elapsed)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.container == nil || f.volume == nil || f.network == nil {
		t.Error("cleanup mutated after deadline")
	}
}

func TestCleanupIsBoundedWhileGracefulStopIsBlocked(t *testing.T) {
	f := &mobyFixture{}
	entered := make(chan struct{})
	reviewFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/stop") {
			_, _ = io.Copy(io.Discard, r.Body)
			if r.URL.Query().Get("t") != "-1" {
				t.Error("stop must not escalate to forced kill")
			}
			close(entered)
			<-time.After(400 * time.Millisecond)
		}
		f.ServeHTTP(w, r)
	}))
	s, err := New(context.Background(), config())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fixtureStart(t, s); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	var stopErr error
	go func() { _, stopErr = s.Stop(context.Background()); close(done) }()
	defer func() {
		<-done
		if stopErr != nil {
			t.Error(stopErr)
		}
		if err := s.Cleanup(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("stop request not observed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	started := time.Now()
	err = s.Cleanup(ctx)
	elapsed := time.Since(started)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("want bounded lock-wait error: %v", err)
	}
	if elapsed > 300*time.Millisecond {
		t.Errorf("cleanup waited past its budget behind Stop(ctx.Background()): %s", elapsed)
	}
	select {
	case <-done:
		t.Error("cleanup did not return until blocked graceful Stop completed")
	default:
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.requests {
		if strings.HasPrefix(r, "DELETE ") {
			t.Errorf("concurrent cleanup mutated resources before owning operation gate: %s", r)
		}
	}
}

func TestForeignContainerCollisionPreflightAllocatesNothing(t *testing.T) {
	f := &mobyFixture{container: foreignContainer()}
	fixture(t, f)
	s, err := New(context.Background(), config())
	if s != nil || err == nil {
		t.Fatalf("collision accepted: session=%v err=%v", s, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if !reflect.DeepEqual(f.container, foreignContainer()) {
		t.Fatal("foreign container changed")
	}
	if f.volume != nil || f.network != nil {
		t.Error("foreign name collision leaked owned volume/network")
	}
	for _, request := range f.requests {
		if strings.HasPrefix(request, "POST ") || strings.HasPrefix(request, "DELETE ") {
			t.Errorf("pre-existing container collision mutated Docker: %s", request)
		}
	}
}
