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
