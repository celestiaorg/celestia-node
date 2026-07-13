package rpc

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testRemoteAddr = "1.2.3.4:1234"
	testCacheSize  = 8
)

func TestConnLimit_AllowsWithinLimit(t *testing.T) {
	handler := connLimit(2, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestConnLimit_RejectsOverLimit(t *testing.T) {
	blocked := make(chan struct{})
	released := make(chan struct{})
	handler := connLimit(1, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Block") == "true" {
			close(blocked)
			<-released
		}
		w.WriteHeader(http.StatusOK)
	}))

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Block", "true")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}()

	<-blocked

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	close(released)
	wg.Wait()
}

func TestConnLimit_ReleasesSlotAfterRequest(t *testing.T) {
	handler := connLimit(1, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRateLimit_AllowsWithinLimit(t *testing.T) {
	handler := rateLimit(100, 10, testCacheSize, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = testRemoteAddr
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRateLimit_RejectsOverBurst(t *testing.T) {
	handler := rateLimit(1, 3, testCacheSize, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = testRemoteAddr

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "request %d should succeed", i)
	}

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestRateLimit_DifferentIPsIndependent(t *testing.T) {
	handler := rateLimit(1, 1, testCacheSize, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req1 := httptest.NewRequest("GET", "/", nil)
	req1.RemoteAddr = testRemoteAddr
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req1)
	assert.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req1)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "5.6.7.8:5678"
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req2)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRateLimit_EvictedIPGetsFreshBurst verifies that when the LRU cache fills
// up and an IP's limiter is evicted, the IP gets a brand-new bucket (full burst)
// on its next request — not the old, exhausted state.
func TestRateLimit_EvictedIPGetsFreshBurst(t *testing.T) {
	const cacheSize = 2
	handler := rateLimit(1, 1, cacheSize, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	do := func(remoteAddr string) int {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = remoteAddr
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w.Code
	}

	// Drain IP A's burst.
	assert.Equal(t, http.StatusOK, do("1.1.1.1:1"))
	assert.Equal(t, http.StatusTooManyRequests, do("1.1.1.1:1"))

	// Fill the cache past capacity with two more IPs, evicting A.
	assert.Equal(t, http.StatusOK, do("2.2.2.2:1"))
	assert.Equal(t, http.StatusOK, do("3.3.3.3:1"))

	// A is evicted — next request from A should get a fresh limiter, not 429.
	assert.Equal(t, http.StatusOK, do("1.1.1.1:1"))
}

func TestIsWebsocketUpgrade(t *testing.T) {
	tests := []struct {
		name       string
		upgrade    string
		connection string
		want       bool
	}{
		{"standard handshake", "websocket", "Upgrade", true},
		{"case-insensitive, listed connection", "WebSocket", "keep-alive, upgrade", true},
		{"plain http", "", "", false},
		{"upgrade header only", "websocket", "", false},
		{"connection header only", "", "Upgrade", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			if tt.upgrade != "" {
				r.Header.Set("Upgrade", tt.upgrade)
			}
			if tt.connection != "" {
				r.Header.Set("Connection", tt.connection)
			}
			assert.Equal(t, tt.want, isWebsocketUpgrade(r))
		})
	}
}

// TestTrackWebsocket_BracketsConnection verifies the live-ws gauge is +1 while
// the (blocking) ws handler runs and back to 0 once it returns — the close
// signal net/http never delivers for hijacked connections.
func TestTrackWebsocket_BracketsConnection(t *testing.T) {
	m := &rpcMetrics{}

	inHandler := make(chan struct{})
	release := make(chan struct{})
	handler := m.trackWebsocket(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		close(inHandler)
		<-release // mimic a ws read-loop blocking for the connection lifetime
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}()

	<-inHandler
	assert.Equal(t, int64(1), m.websocketConnsOpen.Load(), "gauge should be incremented while ws is open")

	close(release)
	wg.Wait()
	assert.Equal(t, int64(0), m.websocketConnsOpen.Load(), "gauge should be decremented after ws closes")
}

func TestTrackWebsocket_IgnoresPlainHTTP(t *testing.T) {
	m := &rpcMetrics{}
	handler := m.trackWebsocket(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		assert.Equal(t, int64(0), m.websocketConnsOpen.Load(), "plain HTTP must not touch the ws gauge")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(0), m.websocketConnsOpen.Load())
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		remoteAddr string
		expected   string
	}{
		{testRemoteAddr, "1.2.3.4"},
		{"[::1]:1234", "::1"},
		{"1.2.3.4", "1.2.3.4"},
	}
	for _, tt := range tests {
		r := &http.Request{RemoteAddr: tt.remoteAddr}
		assert.Equal(t, tt.expected, extractIP(r))
	}
}
