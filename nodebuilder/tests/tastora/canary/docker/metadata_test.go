package docker

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// metadataReader is all the engine can call for node metadata: two reads.
type metadataReader interface {
	P2PInfo(context.Context) (peer.AddrInfo, error)
	P2PPeers(context.Context) ([]peer.ID, error)
}

func TestMetadataOnlyHardcodedReadsWithShortLivedInternalAuth(t *testing.T) {
	key, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id, err := peer.IDFromPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var methods, auth []string
	rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		method, _ := req["method"].(string)
		mu.Lock()
		methods = append(methods, method)
		auth = append(auth, r.Header.Get("Authorization"))
		mu.Unlock()
		var result any
		switch method {
		case "p2p.Info":
			result = peer.AddrInfo{ID: id}
		case "p2p.Peers":
			result = []peer.ID{id}
		case "header.SyncState":
			result = map[string]any{}
		default:
			t.Error("unexpected mutation/arbitrary RPC operation")
			w.WriteHeader(403)
			return
		}
		reply(w, map[string]any{"jsonrpc": "2.0", "id": req["id"], "result": result})
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
	reader, ok := any(s).(metadataReader)
	if !ok {
		t.Fatal("Session lacks read-only native P2P metadata operations")
	}
	info, err := reader.P2PInfo(context.Background())
	if err != nil || info.ID != id {
		t.Fatal("native Info decoding failed")
	}
	peers, err := reader.P2PPeers(context.Background())
	if err != nil || len(peers) != 1 || peers[0] != id {
		t.Fatal("native Peers decoding failed")
	}
	c, err := s.RPC(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err = c.Header.SyncState(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(methods, []string{"p2p.Info", "p2p.Peers", "header.SyncState"}) {
		t.Fatalf("unexpected operations: %v", methods)
	}
	if !reflect.DeepEqual(
		auth,
		[]string{"Bearer fixture.admin.1", "Bearer fixture.admin.2", "Bearer fixture.read.token"},
	) {
		t.Fatal("scoped admin authorization was reused or leaked to read RPC")
	}
	if len(f.execHistory) != 3 {
		t.Fatal("metadata authorization cached beyond operation")
	}
	for i, ex := range f.execHistory {
		permission, ttl := "admin", "30s"
		if i == 2 {
			permission, ttl = "read", "1h"
		}
		want := []any{
			"/bin/celestia",
			"light",
			"auth",
			permission,
			"--node.store",
			storePath,
			"--p2p.network",
			"mocha",
			"--ttl",
			ttl,
		}
		if !reflect.DeepEqual(ex["Cmd"], want) || ex["Tty"] == true || ex["Privileged"] == true {
			t.Fatal("unsafe native authorization execution")
		}
	}
	// Metadata methods take no arguments and only read.
	typ := reflect.TypeOf(s)
	for _, name := range []string{"P2PInfo", "P2PPeers"} {
		m, ok := typ.MethodByName(name)
		if !ok || m.Type.NumIn() != 2 {
			t.Fatalf("unsafe exported metadata API: %s", name)
		}
	}
	for i := 0; i < typ.NumMethod(); i++ {
		name := typ.Method(i).Name
		if strings.Contains(strings.ToLower(name), "admin") {
			t.Fatal("admin client exposed")
		}
	}
}

func TestMetadataRefusesUnobservedBootBeforeMinting(t *testing.T) {
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
	f.container["State"].(map[string]any)["StartedAt"] = "2026-09-08T01:00:10Z"
	if _, err = s.P2PInfo(context.Background()); err == nil {
		t.Fatal("unobserved boot metadata accepted")
	}
	if _, err = s.P2PPeers(context.Background()); err == nil {
		t.Fatal("unobserved boot peers accepted")
	}
	if f.exec != nil {
		t.Fatal("admin authorization minted for unobserved boot")
	}
}

func TestMetadataFailuresAreRedactedAndBounded(t *testing.T) {
	const (
		kindRPCError    = "RPC error"
		kindRPCCancel   = "RPC cancellation"
		kindExecFailure = "exec failure"
		kindExecCancel  = "exec cancellation"
		kindWildcard    = "wildcard"
		kindOwner       = "owner"
		kindStopped     = "stopped"
		kindClosed      = "closed"
	)
	for _, kind := range []string{
		kindRPCError,
		kindRPCCancel,
		kindExecFailure,
		kindExecCancel,
		kindWildcard,
		kindOwner,
		kindStopped,
		kindClosed,
	} {
		t.Run(kind, func(t *testing.T) {
			rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req map[string]any
				_ = json.NewDecoder(r.Body).Decode(&req)
				if kind == kindRPCCancel {
					<-r.Context().Done()
					return
				}
				reply(
					w,
					map[string]any{
						"jsonrpc": "2.0",
						"id":      req["id"],
						"error": map[string]any{
							"code":    -32000,
							"message": "fixture.admin.1 http://secret.invalid underlying failure",
						},
					},
				)
			}))
			defer rpc.Close()
			f := &mobyFixture{
				rpcAddress: strings.TrimPrefix(rpc.URL, "http://"),
				execFail:   kind == kindExecFailure,
				tokenHang:  kind == kindExecCancel,
			}
			fixture(t, f)
			s, err := New(context.Background(), config())
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if kind == kindOwner && f.container != nil {
					f.container["Config"].(map[string]any)["Labels"] = s.labels()
				}
				_ = s.Cleanup(context.Background())
			}()
			if _, err = fixtureStart(t, s); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case kindWildcard:
				f.rpcAddress = "0.0.0.0:12345"
			case kindOwner:
				f.container["Config"].(map[string]any)["Labels"] = map[string]string{runLabel: "foreign"}
			case kindStopped:
				if _, err = s.Stop(context.Background()); err != nil {
					t.Fatal(err)
				}
			case kindClosed:
				if err = s.Cleanup(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			for _, method := range []string{"Info", "Peers"} {
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				want := "native P2P info unavailable"
				if method == "Info" {
					_, err = s.P2PInfo(ctx)
				} else {
					_, err = s.P2PPeers(ctx)
					want = "native P2P peers unavailable"
				}
				cancel()
				if err == nil || err.Error() != want {
					t.Fatal("metadata failure not exactly redacted")
				}
			}
			if (kind == kindWildcard || kind == kindOwner || kind == kindStopped || kind == kindClosed) && f.exec != nil {
				t.Fatal("authorization minted before identity/state/binding validation")
			}
		})
	}
}

func TestMetadataConcurrentCallsAndCleanup(t *testing.T) {
	rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		reply(w, map[string]any{"jsonrpc": "2.0", "id": req["id"], "result": []string{}})
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
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Go(func() {
			_, err := s.P2PPeers(context.Background())
			if err != nil && err.Error() != "native P2P peers unavailable" {
				t.Error("unsanitized concurrent metadata failure")
			}
		})
	}
	wg.Go(func() {
		if err := s.Cleanup(context.Background()); err != nil {
			t.Error(err)
		}
	})
	wg.Wait()
}
