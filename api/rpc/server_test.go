package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/cristalhq/jwt/v5"
	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/api/rpc/perms"
)

// TestServer_HandlerStackSelection tests that the correct middleware stack is selected
func TestServer_HandlerStackSelection(t *testing.T) {
	signer, verifier := createTestJWT(t)

	tests := []struct {
		name         string
		authDisabled bool
		corsEnabled  bool
		expectCORS   bool
		expectAuth   bool
	}{
		{
			name:         "Auth disabled - should use permissive CORS",
			authDisabled: true,
			corsEnabled:  false,
			expectCORS:   true,
			expectAuth:   false,
		},
		{
			name:         "Auth enabled, CORS enabled - should use both",
			authDisabled: false,
			corsEnabled:  true,
			expectCORS:   true,
			expectAuth:   true,
		},
		{
			name:         "Auth enabled, CORS disabled - should use auth only",
			authDisabled: false,
			corsEnabled:  false,
			expectCORS:   false,
			expectAuth:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			corsConfig := CORSConfig{
				Enabled:        tt.corsEnabled,
				AllowedOrigins: []string{"https://example.com"},
				AllowedMethods: []string{"GET", "POST"},
				AllowedHeaders: []string{"Content-Type"},
			}

			server := NewServer("localhost", "0", tt.authDisabled, corsConfig, signer, verifier)

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := server.newHandlerStack(testHandler)

			// Test OPTIONS request (CORS preflight)
			req := httptest.NewRequest("OPTIONS", "/", nil)
			req.Header.Set("Origin", "https://example.com")
			req.Header.Set("Access-Control-Request-Method", "POST")

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if tt.expectCORS {
				assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Origin"))
			} else {
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
			}

			req = httptest.NewRequest("GET", "/", nil)
			if tt.expectAuth {
				w = httptest.NewRecorder()
				handler.ServeHTTP(w, req)
				assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusUnauthorized)
			}
		})
	}
}

// TestServer_AuthDisabledOverridesCORS tests that auth disabled always uses permissive CORS
func TestServer_AuthDisabledOverridesCORS(t *testing.T) {
	signer, verifier := createTestJWT(t)

	// Even with restrictive CORS config, auth disabled should allow all
	restrictiveCORS := CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://only-this-origin.com"},
		AllowedMethods: []string{"GET"},
		AllowedHeaders: []string{"Content-Type"},
	}

	server := NewServer("localhost", "0", true, restrictiveCORS, signer, verifier)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := server.newHandlerStack(testHandler)

	// Test with different origin than configured
	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://different-origin.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}

// TestServer_CORSConfigurationPassing tests that CORS config is passed correctly
func TestServer_CORSConfigurationPassing(t *testing.T) {
	signer, verifier := createTestJWT(t)

	tests := []struct {
		name       string
		corsConfig CORSConfig
		testOrigin string
		expectPass bool
	}{
		{
			name: "Specific origin allowed",
			corsConfig: CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"https://example.com"},
				AllowedMethods: []string{"GET", "POST"},
				AllowedHeaders: []string{"Content-Type"},
			},
			testOrigin: "https://example.com",
			expectPass: true,
		},
		{
			name: "Wildcard origin",
			corsConfig: CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"*"},
				AllowedMethods: []string{"GET", "POST"},
				AllowedHeaders: []string{"Content-Type"},
			},
			testOrigin: "https://any-origin.com",
			expectPass: true,
		},
		{
			name: "Multiple origins",
			corsConfig: CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"https://first.com", "https://second.com"},
				AllowedMethods: []string{"GET", "POST"},
				AllowedHeaders: []string{"Content-Type"},
			},
			testOrigin: "https://second.com",
			expectPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer("localhost", "0", false, tt.corsConfig, signer, verifier)

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := server.newHandlerStack(testHandler)

			req := httptest.NewRequest("OPTIONS", "/", nil)
			req.Header.Set("Origin", tt.testOrigin)
			req.Header.Set("Access-Control-Request-Method", "POST")

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if tt.expectPass {
				origin := w.Header().Get("Access-Control-Allow-Origin")
				assert.True(t, origin == tt.testOrigin || origin == "*")
			}
		})
	}
}

// TestServer_AuthMiddleware tests authentication behavior
func TestServer_AuthMiddleware(t *testing.T) {
	signer, verifier := createTestJWT(t)

	tests := []struct {
		name         string
		authDisabled bool
		token        string
		expectedCode int
	}{
		{
			name:         "Auth disabled - no token required",
			authDisabled: true,
			token:        "",
			expectedCode: http.StatusOK,
		},
		{
			name:         "Auth enabled - valid token",
			authDisabled: false,
			token:        createValidToken(t, signer, perms.ReadPerms),
			expectedCode: http.StatusOK,
		},
		{
			name:         "Auth enabled - no token (read-only access)",
			authDisabled: false,
			token:        "",
			expectedCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer("localhost", "0", tt.authDisabled, CORSConfig{}, signer, verifier)

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := server.newHandlerStack(testHandler)

			req := httptest.NewRequest("GET", "/", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)
		})
	}
}

// TestServer_VerifyAuth tests the auth verification logic
func TestServer_VerifyAuth(t *testing.T) {
	signer, verifier := createTestJWT(t)

	tests := []struct {
		name          string
		authDisabled  bool
		token         string
		expectedPerms []auth.Permission
		expectError   bool
	}{
		{
			name:          "Auth disabled - returns all permissions",
			authDisabled:  true,
			token:         "",
			expectedPerms: perms.AllPerms,
			expectError:   false,
		},
		{
			name:          "Valid token",
			authDisabled:  false,
			token:         createValidToken(t, signer, perms.ReadPerms),
			expectedPerms: perms.ReadPerms,
			expectError:   false,
		},
		{
			name:         "Invalid token",
			authDisabled: false,
			token:        "invalid-token",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer("localhost", "0", tt.authDisabled, CORSConfig{}, signer, verifier)

			permissions, err := server.verifyAuth(context.Background(), tt.token)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPerms, permissions)
			}
		})
	}
}

// TestServer_StartStop tests server lifecycle
func TestServer_StartStop(t *testing.T) {
	signer, verifier := createTestJWT(t)
	server := NewServer("localhost", "0", false, CORSConfig{}, signer, verifier)

	ctx := context.Background()

	err := server.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, server.started.Load())
	assert.NotEmpty(t, server.ListenAddr())

	err = server.Start(ctx)
	assert.NoError(t, err)

	err = server.Stop(ctx)
	assert.NoError(t, err)
	assert.False(t, server.started.Load())
	assert.Empty(t, server.ListenAddr())

	err = server.Stop(ctx)
	assert.NoError(t, err)
}

// TestServer_BatchRequestLimit verifies that the RPC server limits the number
// of requests in a JSON-RPC batch. Without a limit, an attacker can send a
// small HTTP POST containing thousands of method calls that each trigger large
// responses, causing the node to allocate gigabytes of memory and OOM crash.
//
// See: CELESTIA-202 — Unlimited JSON-RPC Batch Requests to Crash celestia-node
func TestServer_BatchRequestLimit(t *testing.T) {
	signer, verifier := createTestJWT(t)
	server := NewServer("localhost", "0", true, CORSConfig{}, signer, verifier)

	// Register a simple echo service so the RPC server has a valid method to call.
	server.RegisterService("test", &testService{}, &testServiceAPI{})

	ctx := context.Background()
	err := server.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { server.Stop(ctx) })

	addr := server.ListenAddr()
	client := &http.Client{Timeout: 10 * time.Second}

	// Build a batch with a large number of requests. Each individual request is
	// tiny, but 10,000 of them in a batch can cause significant memory pressure
	// if responses are large (e.g., share.GetEDS returns megabytes per call).
	const batchSize = 10_000
	batch := buildJSONRPCBatch(t, batchSize, "test.Echo", []string{"hello"})

	resp, err := client.Post("http://"+addr, "application/json", bytes.NewReader(batch))
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// The server should reject excessively large batches. A well-configured
	// server would either:
	// 1. Return an error for batches exceeding a max batch size, or
	// 2. Limit the request body size so large batches are rejected.
	//
	// Currently the server processes all 10,000 requests without any limit,
	// which confirms the vulnerability. This test should start failing once
	// a batch size limit is implemented.
	var responses []json.RawMessage
	if err := json.Unmarshal(body, &responses); err == nil {
		// If we got here, all batch requests were processed — vulnerability is present.
		if len(responses) == batchSize {
			t.Errorf("VULNERABILITY: server processed all %d batch requests without limit. "+
				"An attacker can use this to cause OOM by sending batch requests for "+
				"expensive methods like share.GetEDS", batchSize)
		}
	}
	// If unmarshalling failed or we got fewer responses, a limit may be in place.
}

func buildJSONRPCBatch(t *testing.T, count int, method string, params any) []byte {
	t.Helper()
	type rpcReq struct {
		JSONRPC string `json:"jsonrpc"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
		ID      int    `json:"id"`
	}
	reqs := make([]rpcReq, count)
	for i := range reqs {
		reqs[i] = rpcReq{JSONRPC: "2.0", Method: method, Params: params, ID: i}
	}
	data, err := json.Marshal(reqs)
	require.NoError(t, err)
	return data
}

// TestServer_BatchResponseAmplification demonstrates the memory amplification
// attack. A method that returns a large response (simulating share.GetEDS which
// returns full Extended Data Squares) is called many times in a single batch.
// The server accumulates all responses in memory before sending, so a small
// request causes massive server-side allocation.
//
// See: CELESTIA-202 — Unlimited JSON-RPC Batch Requests to Crash celestia-node
func TestServer_BatchResponseAmplification(t *testing.T) {
	signer, verifier := createTestJWT(t)
	server := NewServer("localhost", "0", true, CORSConfig{}, signer, verifier)

	server.RegisterService("test", &testService{}, &testServiceAPI{})

	ctx := context.Background()
	err := server.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { server.Stop(ctx) })

	addr := server.ListenAddr()
	client := &http.Client{Timeout: 30 * time.Second}

	// Each call to BigResponse returns 512 KB. With 500 requests in a batch,
	// the server must hold ~256 MB of response data in memory before flushing.
	// A real attacker would use share.GetEDS (~8 MB per response on mainnet)
	// with 20,000 requests, causing ~160 GB of allocation.
	const batchSize = 500
	batch := buildJSONRPCBatch(t, batchSize, "test.BigResponse", []int{512 * 1024})

	requestSize := len(batch)

	// Measure heap allocation caused by processing the batch.
	var memBefore, memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	resp, err := client.Post("http://"+addr, "application/json", bytes.NewReader(batch))
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	runtime.ReadMemStats(&memAfter)

	responseSize := len(body)
	amplification := float64(responseSize) / float64(requestSize)

	t.Logf("Request size:  %s (%d bytes)", formatBytes(requestSize), requestSize)
	t.Logf("Response size: %s (%d bytes)", formatBytes(responseSize), responseSize)
	t.Logf("Amplification: %.0fx", amplification)
	t.Logf("Heap increase: %s", formatBytes(int(memAfter.TotalAlloc-memBefore.TotalAlloc)))

	// Verify the server actually processed the full batch (vulnerability present).
	var responses []json.RawMessage
	err = json.Unmarshal(body, &responses)
	require.NoError(t, err)

	if len(responses) == batchSize {
		t.Errorf("VULNERABILITY: server processed batch of %d large-response requests. "+
			"Request was %s but response was %s (%.0fx amplification). "+
			"An attacker using share.GetEDS (~8MB/response) with 20K requests "+
			"would cause ~160GB allocation → OOM crash.",
			batchSize, formatBytes(requestSize), formatBytes(responseSize), amplification)
	}
}

func formatBytes(b int) string {
	switch {
	case b >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// testService is a minimal RPC service used for testing.
type testService struct{}

func (s *testService) Echo(_ context.Context, msg string) (string, error) {
	return msg, nil
}

// BigResponse returns a byte slice of the requested size, simulating an
// expensive RPC method like share.GetEDS that returns large data.
func (s *testService) BigResponse(_ context.Context, size int) ([]byte, error) {
	return make([]byte, size), nil
}

// testServiceAPI mirrors testService for go-jsonrpc's permissioned proxy pattern.
type testServiceAPI struct {
	Internal struct {
		Echo        func(ctx context.Context, msg string) (string, error)  `perm:"public"`
		BigResponse func(ctx context.Context, size int) ([]byte, error)    `perm:"public"`
	}
}

func createTestJWT(t *testing.T) (jwt.Signer, jwt.Verifier) {
	key := []byte("test-secret-key-for-jwt-signing-32")
	signer, err := jwt.NewSignerHS(jwt.HS256, key)
	require.NoError(t, err)

	verifier, err := jwt.NewVerifierHS(jwt.HS256, key)
	require.NoError(t, err)

	return signer, verifier
}

func createValidToken(t *testing.T, signer jwt.Signer, permissions []auth.Permission) string {
	token, err := perms.NewTokenWithTTL(signer, permissions, time.Hour)
	require.NoError(t, err)
	return string(token)
}
