package rpc

import (
	"context"
	"net/http"
	"net/http/httptest"
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
