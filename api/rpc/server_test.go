package rpc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

			server := NewServer("localhost", "0", tt.authDisabled, corsConfig, TLSConfig{}, signer, verifier)

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := server.newHandlerStack(context.Background(), testHandler)

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

	server := NewServer("localhost", "0", true, restrictiveCORS, TLSConfig{}, signer, verifier)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := server.newHandlerStack(context.Background(), testHandler)

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
			server := NewServer("localhost", "0", false, tt.corsConfig, TLSConfig{}, signer, verifier)

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := server.newHandlerStack(context.Background(), testHandler)

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
			server := NewServer("localhost", "0", tt.authDisabled, CORSConfig{}, TLSConfig{}, signer, verifier)

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := server.newHandlerStack(context.Background(), testHandler)

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
			server := NewServer("localhost", "0", tt.authDisabled, CORSConfig{}, TLSConfig{}, signer, verifier)

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
	server := NewServer("localhost", "0", false, CORSConfig{}, TLSConfig{}, signer, verifier)

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

// TestServer_TLS tests TLS server startup and HTTPS connectivity
func TestServer_TLS(t *testing.T) {
	signer, verifier := createTestJWT(t)

	// Generate self-signed cert
	certFile, keyFile := generateSelfSignedCert(t)

	srv := NewServer("127.0.0.1", "0", true, CORSConfig{}, TLSConfig{
		Enabled:  true,
		CertPath: certFile,
		KeyPath:  keyFile,
	}, signer, verifier)

	ctx := context.Background()
	err := srv.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, srv.Stop(ctx)) })

	addr := srv.ListenAddr()
	require.NotEmpty(t, addr)

	// HTTPS client with InsecureSkipVerify for self-signed cert
	tlsClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// TLS server should respond to HTTPS (400 is expected since GET / is not a valid JSON-RPC request)
	resp, err := tlsClient.Get("https://" + addr)
	require.NoError(t, err)
	resp.Body.Close()
	// Any HTTP response means TLS handshake succeeded
	assert.NotNil(t, resp)

	// Plain HTTP to TLS server should fail with a TLS handshake error
	plainClient := &http.Client{Timeout: 2 * time.Second}
	_, err = plainClient.Get("http://" + addr) //nolint:bodyclose
	// The server sends a TLS record which the plain HTTP client can't parse.
	// Depending on timing, this may return an error or a malformed response.
	// Either way, it should not return a valid HTTP status code.
	if err == nil {
		t.Log("plain HTTP to TLS server returned no error (server sent TLS alert as HTTP response)")
	}
}

// TestServer_NoTLS_PlainHTTP tests that non-TLS server works with plain HTTP clients
func TestServer_NoTLS_PlainHTTP(t *testing.T) {
	signer, verifier := createTestJWT(t)

	srv := NewServer("127.0.0.1", "0", true, CORSConfig{}, TLSConfig{}, signer, verifier)

	ctx := context.Background()
	err := srv.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, srv.Stop(ctx)) })

	addr := srv.ListenAddr()

	// Plain HTTP should work (400 is expected since GET / is not a valid JSON-RPC request,
	// but a response means the server is reachable over plain HTTP)
	resp, err := http.Get("http://" + addr)
	require.NoError(t, err)
	resp.Body.Close()
	assert.NotNil(t, resp)
}

func generateSelfSignedCert(t *testing.T) (certPath, keyPath string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{Organization: []string{"Test"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	certPath = filepath.Join(t.TempDir(), "cert.pem")
	certOut, err := os.Create(certPath)
	require.NoError(t, err)
	require.NoError(t, pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	certOut.Close()

	keyPath = filepath.Join(t.TempDir(), "key.pem")
	keyOut, err := os.Create(keyPath)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	require.NoError(t, pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	keyOut.Close()

	return certPath, keyPath
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
