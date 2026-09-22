package docker

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moby/moby/api/pkg/stdcopy"

	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
)

const fixtureImage = "registry.example/node@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// These tests run the Docker SDK against a fake HTTP daemon.
// Fixture values the Docker API fake checks.
const (
	// authAdmin is the RPC permission a canary must never mint.
	authAdmin = "admin"
	// borrowedNetworkID names an existing network a local run may join.
	borrowedNetworkID = "borrowed"
)

type mobyFixture struct {
	mu            sync.Mutex
	requests      []string
	volumeExists  bool
	volume        map[string]any
	network       map[string]any
	container     map[string]any
	create        map[string]any
	failCreate    bool
	failStart     bool
	stopExit      int
	oom           bool
	attachConn    net.Conn
	attachQueries []string
	attachFail    bool
	attachCalls   int
	starts        int
	stopTimeout   string
	rpcAddress    string
	exec          map[string]any
	execHistory   []map[string]any
	adminMints    int
	execFail      bool
	cancelCreate  context.CancelFunc
	tokenHang     bool
	tokenPrefix   string
	pullError     bool
	wrongDigest   bool
	imageMissing  bool
}

func writeFixtureFrame(w io.Writer, payload string) {
	frame := make([]byte, 8)
	frame[0] = 2
	binary.BigEndian.PutUint32(frame[4:], uint32(len(payload)))
	_, _ = w.Write(frame)
	_, _ = io.WriteString(w, payload)
}
func reply(w http.ResponseWriter, x any) { _ = json.NewEncoder(w).Encode(x) }
func (f *mobyFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := strings.TrimPrefix(r.URL.Path, "/v1.55")
	f.requests = append(f.requests, r.Method+" "+p)
	w.Header().Set("Content-Type", "application/json")
	if p == "/_ping" {
		w.Header().Set("API-Version", "1.55")
		_, _ = w.Write([]byte("OK"))
		return
	}
	if r.Method == http.MethodGet && strings.HasPrefix(p, "/volumes/") && f.volumeExists {
		reply(w, map[string]any{
			"Name":   "cfn-test-data",
			"Labels": map[string]string{"org.celestia.canary.run": "someone-else"},
		})
		return
	}
	switch {
	case strings.HasSuffix(p, "/exec"):
		f.exec = nil
		_ = json.NewDecoder(r.Body).Decode(&f.exec)
		f.execHistory = append(f.execHistory, f.exec)
		if cmd, ok := f.exec["Cmd"].([]any); ok && len(cmd) > 3 && cmd[3] == authAdmin {
			f.adminMints++
		}
		reply(w, map[string]string{"Id": "exec-owned"})
		return
	case p == "/exec/exec-owned/start":
		_, _ = io.Copy(io.Discard, r.Body)
		conn, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = rw.WriteString(
			"HTTP/1.1 101 UPGRADED\r\nContent-Type: application/vnd.docker.raw-stream\r\n" +
				"Connection: Upgrade\r\nUpgrade: tcp\r\n\r\n",
		)
		token := "fixture.read.token"
		if cmd, ok := f.exec["Cmd"].([]any); ok && len(cmd) > 3 && cmd[3] == authAdmin {
			token = fmt.Sprintf("fixture.admin.%d", f.adminMints)
		}
		payload := []byte(f.tokenPrefix + token + "\n")
		frame := make([]byte, 8)
		frame[0] = 1
		binary.BigEndian.PutUint32(frame[4:], uint32(len(payload)))
		_, _ = rw.Write(frame)
		_, _ = rw.Write(payload)
		rw.Flush()
		if f.tokenHang {
			_, _ = io.Copy(io.Discard, conn)
		}
		return
	case p == "/exec/exec-owned/json":
		code := 0
		if f.execFail {
			code = 1
		}
		reply(
			w,
			map[string]any{"ID": "exec-owned", "ContainerID": "container-owned", "Running": false, "ExitCode": code},
		)
		return
	case strings.HasSuffix(p, "/start") && strings.HasPrefix(p, "/containers/"):
		if f.failStart {
			w.WriteHeader(500)
			reply(w, map[string]string{"message": "start failed"})
			return
		}
		f.starts++
		if f.attachConn != nil {
			writeFixtureFrame(f.attachConn, fmt.Sprintf("birth-%d\n", f.starts))
		}
		f.container["State"] = map[string]any{
			"Running":    true,
			"StartedAt":  fmt.Sprintf("2026-09-08T01:00:%02d.123456789Z", f.starts),
			"FinishedAt": "0001-01-01T00:00:00Z",
		}
		w.WriteHeader(204)
		return
	case strings.HasSuffix(p, "/stop"):
		f.stopTimeout = r.URL.Query().Get("t")
		if f.attachConn != nil {
			writeFixtureFrame(f.attachConn, fmt.Sprintf("stop-%d\n", f.starts))
			f.attachConn.Close()
			f.attachConn = nil
		}
		state := f.container["State"].(map[string]any)
		state["Running"] = false
		state["ExitCode"] = f.stopExit
		state["OOMKilled"] = f.oom
		state["FinishedAt"] = "2026-09-08T01:00:20Z"
		w.WriteHeader(204)
		return
	case strings.HasSuffix(p, "/attach"):
		f.attachCalls++
		f.attachQueries = append(f.attachQueries, r.URL.RawQuery)
		if f.attachFail {
			w.WriteHeader(500)
			reply(w, map[string]string{"message": "attach unavailable"})
			return
		}
		conn, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		f.attachConn = conn
		_, _ = rw.WriteString(
			"HTTP/1.1 101 UPGRADED\r\nContent-Type: application/vnd.docker.multiplexed-stream\r\n" +
				"Connection: Upgrade\r\nUpgrade: tcp\r\n\r\n",
		)
		rw.Flush()
		return
	case strings.HasSuffix(p, "/logs"):
		// The logs endpoint is served only to catch a fallback to log replay.
		w.Header().Set("Content-Type", "application/vnd.docker.raw-stream")
		_, _ = w.Write([]byte{2, 0, 0, 0, 0, 0, 0, 3, '{', '}', '\n'})
		return
	case p == "/images/create":
		if f.pullError {
			reply(w, map[string]any{"errorDetail": map[string]string{"message": "pull failed"}})
			return
		}
		reply(w, map[string]string{"status": "pulled"})
		return
	case strings.HasPrefix(p, "/images/") && strings.HasSuffix(p, "/json"):
		if f.imageMissing {
			w.WriteHeader(404)
			reply(w, map[string]string{"message": "missing"})
			return
		}
		reply(w, map[string]any{"Id": "sha256:" + strings.Repeat("a", 64), "RepoDigests": func() []string {
			if f.wrongDigest {
				return []string{"wrong"}
			}
			return []string{fixtureImage}
		}(), "Config": map[string]any{"Labels": map[string]string{"org.opencontainers.image.revision": "fixture-source"}}})
		return
	case p == "/networks/create":
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.network = map[string]any{
			"Id":     "net-owned",
			"Name":   body["Name"],
			"Driver": "bridge",
			"Labels": body["Labels"],
			"IPAM":   map[string]any{"Config": []any{map[string]string{"Gateway": "127.0.0.1"}}},
		}
		reply(w, map[string]string{"Id": "net-owned"})
		return
	case r.Method == http.MethodGet && strings.HasPrefix(p, "/networks/") && f.network != nil:
		reply(w, f.network)
		return
	case p == "/volumes/create":
		_ = json.NewDecoder(r.Body).Decode(&f.volume)
		reply(w, f.volume)
		return
	case r.Method == http.MethodGet && strings.HasPrefix(p, "/volumes/") && f.volume != nil:
		reply(w, f.volume)
		return
	case p == "/containers/create":
		_ = json.NewDecoder(r.Body).Decode(&f.create)
		if f.container != nil {
			w.WriteHeader(http.StatusConflict)
			reply(w, map[string]string{"message": "container name already in use"})
			return
		}
		if f.failCreate {
			w.WriteHeader(500)
			reply(w, map[string]string{"message": "create failed"})
			return
		}
		f.container = map[string]any{
			"Id":     "container-owned",
			"Image":  "sha256:" + strings.Repeat("a", 64),
			"Config": f.create,
			"State": map[string]any{
				"Running":    false,
				"StartedAt":  "0001-01-01T00:00:00Z",
				"FinishedAt": "0001-01-01T00:00:00Z",
			},
			"Mounts": []any{
				map[string]string{"Type": "volume", "Name": "cfn-test-data", "Destination": "/home/celestia"},
			},
		}
		if f.cancelCreate != nil {
			f.cancelCreate()
			<-r.Context().Done()
			return
		}
		reply(w, map[string]string{"Id": "container-owned"})
		return
	case r.Method == http.MethodGet && strings.HasPrefix(
		p,
		"/containers/",
	) && strings.HasSuffix(p, "/json") && f.container != nil:
		if f.rpcAddress != "" {
			host, port, _ := net.SplitHostPort(f.rpcAddress)
			f.container["NetworkSettings"] = map[string]any{
				"Ports": map[string]any{"26658/tcp": []any{map[string]string{"HostIp": host, "HostPort": port}}},
			}
		}
		reply(w, f.container)
		return
	case r.Method == http.MethodDelete:
		if strings.HasPrefix(p, "/containers/") {
			f.container = nil
			if f.attachConn != nil {
				f.attachConn.Close()
				f.attachConn = nil
			}
		}
		if strings.HasPrefix(p, "/volumes/") {
			f.volume = nil
		}
		if strings.HasPrefix(p, "/networks/") {
			f.network = nil
		}
		w.WriteHeader(204)
		return
	}
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`{"message":"not found"}`))
}

func fixture(t *testing.T, f *mobyFixture) {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	t.Setenv("DOCKER_HOST", "tcp://"+strings.TrimPrefix(srv.URL, "http://"))
	t.Setenv("DOCKER_API_VERSION", "1.55")
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")
}

func config() Config {
	return Config{
		RunID:   "test",
		Profile: model.Profile{Image: fixtureImage, Network: "mocha", SourceCommit: "fixture-source"},
	}
}

func fixtureStart(t *testing.T, s *Session) (model.ProcessInfo, error) {
	t.Helper()
	if s.logs != nil {
		s.logs.Close()
	}
	l, err := s.Logs(context.Background())
	if err != nil {
		return model.ProcessInfo{}, err
	}
	t.Cleanup(func() { l.Close() })
	return s.Start(context.Background())
}

func TestCreatesExactOwnedResources(t *testing.T) {
	f := &mobyFixture{}
	fixture(t, f)
	s, err := New(context.Background(), config())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Cleanup(context.Background()) }()
	store := s.Store()
	if !store.Fresh || store.VolumeID != "cfn-test-data" || store.Owner != "test" {
		t.Fatalf("store identity: %+v", store)
	}
	if f.create == nil {
		t.Fatal("no native container created")
	}
	if f.create["Image"] != "sha256:"+strings.Repeat("a", 64) {
		t.Fatal("container not pinned by inspected image ID")
	}
	hc := f.create["HostConfig"].(map[string]any)
	binding := hc["PortBindings"].(map[string]any)["26658/tcp"].([]any)[0].(map[string]any)
	if binding["HostIp"] != "127.0.0.1" {
		t.Fatalf("RPC exposed: %v", binding)
	}
	cmd := f.create["Cmd"].([]any)
	text := ""
	for _, v := range cmd {
		text += v.(string) + " "
	}
	if !strings.Contains(text, "--rpc.skip-auth=false") || !strings.Contains(text, "exec /bin/celestia") {
		t.Fatal("missing auth or native exec")
	}
	if err := s.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.container != nil || f.network != nil || f.volume != nil {
		t.Fatal("owned resources survived cleanup")
	}
}

func TestRejectUnsafeConfigurationBeforeDockerMutation(t *testing.T) {
	cases := map[string]func(*Config){
		"mutable image":            func(c *Config) { c.Profile.Image = "node:latest" },
		"invalid run ID":           func(c *Config) { c.RunID = "../other" },
		"public argument override": func(c *Config) { c.AdditionalStartArguments = []string{"--rpc.skip-auth"} },
		"local auth override": func(c *Config) {
			c.ExistingNetworkID = borrowedNetworkID
			c.AdditionalStartArguments = []string{"--rpc.skip-auth=true"}
		},
		"public custom network":    func(c *Config) { c.Profile.CustomNetwork = "private:hash:peers" },
		"source identity mismatch": func(c *Config) { c.Profile.SourceCommit = "wrong" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := &mobyFixture{}
			fixture(t, f)
			c := config()
			change(&c)
			s, err := New(context.Background(), c)
			if s != nil {
				_ = s.Cleanup(context.Background())
			}
			if err == nil {
				t.Fatal("unsafe configuration accepted")
			}
			for _, r := range f.requests {
				if strings.HasPrefix(r, "POST /containers") || strings.HasPrefix(r, "POST /networks") ||
					strings.HasPrefix(r, "POST /volumes") {
					t.Fatalf("unsafe config allocated resources: %s", r)
				}
			}
		})
	}
}

func TestNativeGracefulRestartAndEpochLogs(t *testing.T) {
	f := &mobyFixture{}
	fixture(t, f)
	s, err := New(context.Background(), config())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Cleanup(context.Background()) }()
	var first model.ProcessInfo
	for boot := 1; boot <= 2; boot++ {
		stream, err := s.Logs(context.Background())
		if err != nil {
			t.Fatalf("preattach boot %d: %v", boot, err)
		}
		defer stream.Close()
		info, err := s.Start(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !info.Running || info.StartedAt.IsZero() {
			t.Fatal("missing birth proof")
		}
		if boot == 1 {
			first = info
		} else if info.ContainerID != first.ContainerID || info.VolumeID != first.VolumeID ||
			info.ImageID != first.ImageID || !info.StartedAt.After(first.StartedAt) {
			t.Fatal("restart identity drift")
		}
		stopped, err := s.Stop(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if stopped.Running || stopped.ForcedKill || stopped.ExitCode != 0 || stopped.FinishedAt.IsZero() ||
			f.stopTimeout != "-1" {
			t.Fatal("graceful stop not proven")
		}
		var stderr strings.Builder
		if _, err = stdcopy.StdCopy(io.Discard, &stderr, stream); err != nil {
			t.Fatal(err)
		}
		stream.Close()
		if stderr.String() != fmt.Sprintf("birth-%d\nstop-%d\n", boot, boot) {
			t.Fatalf("lifetime stream lost birth/replayed old boot: %q", stderr.String())
		}
	}
	if f.attachCalls != 2 {
		t.Fatal("restart did not preattach separately")
	}
	for _, raw := range f.attachQueries {
		q, _ := url.ParseQuery(raw)
		if q.Get("stream") != "1" || (q.Get("logs") != "" && q.Get("logs") != "0") || q.Get("stdout") != "1" ||
			q.Get("stderr") != "1" ||
			(q.Get("stdin") != "" && q.Get("stdin") != "0") {
			t.Fatalf("unsafe attach options: %s", raw)
		}
	}
	for _, r := range f.requests {
		if strings.HasSuffix(r, "/logs") {
			t.Fatal("ContainerLogs replay used")
		}
	}
}

func TestAbnormalStopIsNeverGraceful(t *testing.T) {
	for _, exit := range []int{1, 137} {
		t.Run(fmt.Sprint(exit), func(t *testing.T) {
			f := &mobyFixture{stopExit: exit}
			fixture(t, f)
			s, err := New(context.Background(), config())
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = s.Cleanup(context.Background()) }()
			if _, err = fixtureStart(t, s); err != nil {
				t.Fatal(err)
			}
			info, err := s.Stop(context.Background())
			if err == nil {
				t.Fatal("abnormal exit classified graceful")
			}
			if info.ForcedKill != (exit == 137) {
				t.Fatal("forced-kill evidence wrong")
			}
			if _, err = fixtureStart(t, s); err == nil {
				t.Fatal("restart after unproven graceful exit accepted")
			}
		})
	}
}

func TestRestartRejectsIdentityDrift(t *testing.T) {
	const (
		driftImage      = "image"
		driftOwner      = "owner"
		driftMount      = "mount"
		driftStaleEpoch = "stale epoch"
	)
	for _, kind := range []string{driftImage, driftOwner, driftMount, driftStaleEpoch} {
		t.Run(kind, func(t *testing.T) {
			f := &mobyFixture{}
			fixture(t, f)
			s, err := New(context.Background(), config())
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if f.container != nil {
					f.container["Config"].(map[string]any)["Labels"] = s.labels()
				}
				_ = s.Cleanup(context.Background())
			}()
			if _, err = fixtureStart(t, s); err != nil {
				t.Fatal(err)
			}
			if _, err = s.Stop(context.Background()); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case driftImage:
				f.container["Image"] = "sha256:" + strings.Repeat("b", 64)
			case driftOwner:
				f.container["Config"].(map[string]any)["Labels"] = map[string]string{runLabel: "other"}
			case driftMount:
				f.container["Mounts"] = []any{}
			case driftStaleEpoch:
				f.starts = 0
			}
			if _, err = fixtureStart(t, s); err == nil {
				t.Fatal("identity drift accepted")
			}
		})
	}
}

func TestRPCUsesNativeReadTokenInMemory(t *testing.T) {
	rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture.read.token" {
			t.Error("missing in-memory read authorization")
			w.WriteHeader(401)
			return
		}
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["method"] != "header.SyncState" {
			t.Error("unexpected non-read RPC")
		}
		reply(w, map[string]any{"jsonrpc": "2.0", "id": req["id"], "result": map[string]any{}})
	}))
	defer rpc.Close()
	f := &mobyFixture{rpcAddress: strings.TrimPrefix(rpc.URL, "http://")}
	fixture(t, f)
	s, err := New(context.Background(), config())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Cleanup(context.Background()) }()
	if _, err = fixtureStart(t, s); err != nil {
		t.Fatal(err)
	}
	c, err := s.RPC(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err = c.Header.SyncState(context.Background()); err != nil {
		t.Fatal(err)
	}
	command := fmt.Sprint(f.exec["Cmd"])
	if !strings.Contains(command, "light auth read") || strings.Contains(command, authAdmin) || f.exec["Tty"] == true ||
		f.exec["Privileged"] == true {
		t.Fatal("unsafe token execution")
	}
}

func TestCanceledCreateCleansAmbiguousOwnedResources(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &mobyFixture{cancelCreate: cancel}
	fixture(t, f)
	s, err := New(ctx, config())
	if s != nil || err == nil {
		t.Fatal("canceled create accepted")
	}
	if f.container != nil || f.volume != nil || f.network != nil {
		t.Fatal("canceled create leaked resources")
	}
}

func TestFailedStartCleansOwnedResources(t *testing.T) {
	f := &mobyFixture{failStart: true}
	fixture(t, f)
	s, err := New(context.Background(), config())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fixtureStart(t, s); err == nil {
		t.Fatal("failed start accepted")
	}
	if f.container != nil || f.volume != nil || f.network != nil {
		t.Fatal("failed start leaked resources")
	}
}

func TestCleanupCanceledContextPreservesBorrowedNetwork(t *testing.T) {
	f := &mobyFixture{
		network: map[string]any{
			"Id":     borrowedNetworkID,
			"Driver": "bridge",
			"Labels": map[string]string{runLabel: "tastora"},
			"IPAM":   map[string]any{"Config": []any{map[string]string{"Gateway": "127.0.0.1"}}},
		},
	}
	fixture(t, f)
	cfg := config()
	cfg.ExistingNetworkID = borrowedNetworkID
	s, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = s.Cleanup(ctx); err != nil {
		t.Fatal(err)
	}
	if f.network == nil || f.container != nil || f.volume != nil {
		t.Fatal("borrowed ownership/cleanup violated")
	}
}

func TestRPCRejectsUnsafeBindingAndRedactsExecFailure(t *testing.T) {
	const (
		wildcard       = "wildcard"
		execFailure    = "exec failure"
		canceledStream = "canceled stream"
	)
	for _, kind := range []string{wildcard, execFailure, canceledStream} {
		t.Run(kind, func(t *testing.T) {
			f := &mobyFixture{rpcAddress: "127.0.0.1:12345"}
			if kind == wildcard {
				f.rpcAddress = "0.0.0.0:12345"
			}
			f.execFail = kind == execFailure
			f.tokenHang = kind == canceledStream
			fixture(t, f)
			s, err := New(context.Background(), config())
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = s.Cleanup(context.Background()) }()
			if _, err = fixtureStart(t, s); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			c, err := s.RPC(ctx)
			if c != nil {
				c.Close()
			}
			if err == nil {
				t.Fatal("unsafe/failed RPC accepted")
			}
			if strings.Contains(err.Error(), "fixture.read.token") {
				t.Fatal("token leaked to error")
			}
			if kind == wildcard && f.exec != nil {
				t.Fatal("token executed before loopback validation")
			}
		})
	}
}

func TestPullFailureAndDigestMismatchAllocateNothing(t *testing.T) {
	for _, kind := range []string{"pull error", "digest mismatch"} {
		t.Run(kind, func(t *testing.T) {
			f := &mobyFixture{pullError: kind == "pull error", wrongDigest: kind == "digest mismatch"}
			fixture(t, f)
			s, err := New(context.Background(), config())
			if s != nil || err == nil {
				t.Fatal("unverified pull accepted")
			}
			if f.network != nil || f.volume != nil || f.container != nil {
				t.Fatal("failed pull allocated resources")
			}
		})
	}
}

func TestCanceledStartRemovesResources(t *testing.T) {
	f := &mobyFixture{}
	fixture(t, f)
	s, err := New(context.Background(), config())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = s.Start(ctx); err == nil {
		t.Fatal("canceled start accepted")
	}
	if f.container != nil || f.network != nil || f.volume != nil {
		t.Fatal("canceled startup leaked resources")
	}
}

func TestFailedCreateCleansNetworkAndVolume(t *testing.T) {
	f := &mobyFixture{failCreate: true}
	fixture(t, f)
	s, err := New(context.Background(), config())
	if s != nil || err == nil {
		t.Fatal("create failure accepted")
	}
	if f.network != nil || f.volume != nil {
		t.Fatal("create failure leaked resources")
	}
}

func TestNativeCustomNetworkNoticeDoesNotBreakReadToken(t *testing.T) {
	f := &mobyFixture{
		rpcAddress: "127.0.0.1:12345",
		tokenPrefix: "\n\nWARNING: Celestia custom network specified. " +
			"Only use this option if the node is freshly created and initialized.\n" +
			"**DO NOT** run a custom network over an already-existing node store!\n\n",
	}
	fixture(t, f)
	s, err := New(context.Background(), config())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Cleanup(context.Background()) }()
	if _, err = fixtureStart(t, s); err != nil {
		t.Fatal(err)
	}
	c, err := s.RPC(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
}

func TestStopRejectsUnobservedBoot(t *testing.T) {
	f := &mobyFixture{}
	fixture(t, f)
	s, err := New(context.Background(), config())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Cleanup(context.Background()) }()
	if _, err = fixtureStart(t, s); err != nil {
		t.Fatal(err)
	}
	f.container["State"].(map[string]any)["StartedAt"] = "2026-09-08T01:00:10Z"
	if _, err = s.Stop(context.Background()); err == nil {
		t.Fatal("unobserved external restart classified graceful")
	}
	if f.stopTimeout != "" {
		t.Fatal("signaled an unobserved boot")
	}
}

func TestOOMExitZeroIsNotGraceful(t *testing.T) {
	f := &mobyFixture{oom: true}
	fixture(t, f)
	s, err := New(context.Background(), config())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Cleanup(context.Background()) }()
	if _, err = fixtureStart(t, s); err != nil {
		t.Fatal(err)
	}
	info, err := s.Stop(context.Background())
	if err == nil || !info.OOMKilled {
		t.Fatal("OOM incorrectly classified graceful")
	}
}

func TestFreshVolumeCollisionDoesNotDeleteExisting(t *testing.T) {
	f := &mobyFixture{volumeExists: true}
	fixture(t, f)
	s, err := New(context.Background(), config())
	if s != nil || err == nil || !strings.Contains(err.Error(), "volume already exists") {
		t.Fatalf("want fresh-volume collision, session=%v err=%v", s, err)
	}
	for _, r := range f.requests {
		if strings.HasPrefix(r, "DELETE ") || strings.HasPrefix(r, "POST ") {
			t.Fatalf("collision mutated Docker: %s", r)
		}
	}
}
