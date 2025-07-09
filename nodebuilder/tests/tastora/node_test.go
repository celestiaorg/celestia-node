//go:build integration

package tastora

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/stretchr/testify/suite"

	tastoratypes "github.com/celestiaorg/tastora/framework/types"
)

// NodeTestSuite provides comprehensive testing of the Node module APIs.
// Tests node management, meta APIs, authentication, and administrative functionality.
type NodeTestSuite struct {
	suite.Suite
	framework *Framework
}

func TestNodeTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Node module integration tests in short mode")
	}
	suite.Run(t, &NodeTestSuite{})
}

func (s *NodeTestSuite) SetupSuite() {
	s.framework = NewFramework(s.T(), WithValidators(1), WithFullNodes(1), WithBridgeNodes(1), WithLightNodes(1))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
}

func (s *NodeTestSuite) TearDownSuite() {
	if s.framework != nil {
		s.framework.cleanup()
	}
}

// TestNodeInfo validates Node Info API functionality
func (s *NodeTestSuite) TestNodeInfo_FullNode() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Get node info
	info, err := client.Node.Info(ctx)
	s.Require().NoError(err, "should retrieve node info")
	s.Require().NotNil(info, "node info should not be nil")

	// Validate info structure
	s.Assert().NotEmpty(info.Type.String(), "node type should not be empty")
	s.Assert().Equal("Full", info.Type.String(), "should be Full node type")
	s.Assert().NotEmpty(info.APIVersion, "API version should not be empty")

	// Validate API version format
	s.Assert().True(len(info.APIVersion) > 0, "API version should have content")
}

func (s *NodeTestSuite) TestNodeInfo_BridgeNode() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	// Get node info
	info, err := client.Node.Info(ctx)
	s.Require().NoError(err, "should retrieve node info")
	s.Require().NotNil(info, "node info should not be nil")

	// Validate info structure
	s.Assert().Equal("Bridge", info.Type.String(), "should be Bridge node type")
	s.Assert().NotEmpty(info.APIVersion, "API version should not be empty")
}

func (s *NodeTestSuite) TestNodeInfo_LightNode() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	lightNode := s.framework.GetLightNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, lightNode)

	// Get node info
	info, err := client.Node.Info(ctx)
	s.Require().NoError(err, "should retrieve node info")
	s.Require().NotNil(info, "node info should not be nil")

	// Validate info structure
	s.Assert().Equal("Light", info.Type.String(), "should be Light node type")
	s.Assert().NotEmpty(info.APIVersion, "API version should not be empty")
}

// TestNodeReady validates Node Ready API functionality
func (s *NodeTestSuite) TestNodeReady_AllNodeTypes() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test all node types
	nodes := []struct {
		name string
		node tastoratypes.DANode
	}{
		{"full", s.framework.GetFullNodes()[0]},
		{"bridge", s.framework.GetBridgeNodes()[0]},
		{"light", s.framework.GetLightNodes()[0]},
	}

	for _, nodeData := range nodes {
		s.Run(nodeData.name+"_node", func() {
			client := s.framework.GetNodeRPCClient(ctx, nodeData.node)

			// Check if node is ready
			ready, err := client.Node.Ready(ctx)
			s.Require().NoError(err, "should check ready status for %s node", nodeData.name)
			s.Assert().True(ready, "%s node should be ready", nodeData.name)
		})
	}
}

// TestNodeAuthNew validates AuthNew API functionality
func (s *NodeTestSuite) TestNodeAuthNew_ValidPermissions() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Test creating token with read permissions
	readPerms := []auth.Permission{"read"}
	token, err := client.Node.AuthNew(ctx, readPerms)
	s.Require().NoError(err, "should create auth token with read permissions")
	s.Require().NotEmpty(token, "auth token should not be empty")

	// Validate token format (JWT should have 3 parts separated by dots)
	tokenParts := strings.Split(token, ".")
	s.Assert().Equal(3, len(tokenParts), "JWT should have 3 parts")
	s.Assert().True(len(tokenParts[0]) > 0, "JWT header should not be empty")
	s.Assert().True(len(tokenParts[1]) > 0, "JWT payload should not be empty")
	s.Assert().True(len(tokenParts[2]) > 0, "JWT signature should not be empty")
}

func (s *NodeTestSuite) TestNodeAuthNew_AdminPermissions() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Test creating token with admin permissions
	adminPerms := []auth.Permission{"admin"}
	token, err := client.Node.AuthNew(ctx, adminPerms)
	s.Require().NoError(err, "should create auth token with admin permissions")
	s.Require().NotEmpty(token, "auth token should not be empty")

	// Validate token format
	tokenParts := strings.Split(token, ".")
	s.Assert().Equal(3, len(tokenParts), "JWT should have 3 parts")
}

func (s *NodeTestSuite) TestNodeAuthNew_MultiplePermissions() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Test creating token with multiple permissions
	multiPerms := []auth.Permission{"read", "admin"}
	token, err := client.Node.AuthNew(ctx, multiPerms)
	s.Require().NoError(err, "should create auth token with multiple permissions")
	s.Require().NotEmpty(token, "auth token should not be empty")
}

// TestNodeAuthVerify validates AuthVerify API functionality
func (s *NodeTestSuite) TestNodeAuthVerify_ValidToken() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Create a token first
	originalPerms := []auth.Permission{"read", "admin"}
	token, err := client.Node.AuthNew(ctx, originalPerms)
	s.Require().NoError(err, "should create auth token")

	// Verify the token
	verifiedPerms, err := client.Node.AuthVerify(ctx, token)
	s.Require().NoError(err, "should verify auth token")
	s.Require().NotNil(verifiedPerms, "verified permissions should not be nil")

	// Check that permissions match
	s.Assert().Equal(len(originalPerms), len(verifiedPerms), "permission count should match")

	// Convert to strings for comparison
	originalStrs := make([]string, len(originalPerms))
	verifiedStrs := make([]string, len(verifiedPerms))

	for i, perm := range originalPerms {
		originalStrs[i] = string(perm)
	}
	for i, perm := range verifiedPerms {
		verifiedStrs[i] = string(perm)
	}

	s.Assert().ElementsMatch(originalStrs, verifiedStrs, "permissions should match")
}

func (s *NodeTestSuite) TestNodeAuthVerify_InvalidToken() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Test with invalid token
	invalidToken := "invalid.jwt.token"
	_, err := client.Node.AuthVerify(ctx, invalidToken)
	s.Assert().Error(err, "should return error for invalid token")
}

func (s *NodeTestSuite) TestNodeAuthVerify_MalformedToken() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Test with malformed token
	malformedToken := "not.a.valid.jwt"
	_, err := client.Node.AuthVerify(ctx, malformedToken)
	s.Assert().Error(err, "should return error for malformed token")
}

// TestNodeAuthNewWithExpiry validates AuthNewWithExpiry API functionality
func (s *NodeTestSuite) TestNodeAuthNewWithExpiry_ValidTTL() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Create token with expiry
	perms := []auth.Permission{"read"}
	ttl := 5 * time.Minute
	token, err := client.Node.AuthNewWithExpiry(ctx, perms, ttl)
	s.Require().NoError(err, "should create auth token with expiry")
	s.Require().NotEmpty(token, "auth token should not be empty")

	// Verify the token can be used immediately
	verifiedPerms, err := client.Node.AuthVerify(ctx, token)
	s.Require().NoError(err, "should verify newly created token")
	s.Assert().Equal(len(perms), len(verifiedPerms), "permissions should match")
}

func (s *NodeTestSuite) TestNodeAuthNewWithExpiry_ZeroTTL() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Create token with zero TTL (should not expire)
	perms := []auth.Permission{"read"}
	ttl := 0 * time.Second
	token, err := client.Node.AuthNewWithExpiry(ctx, perms, ttl)
	s.Require().NoError(err, "should create auth token with zero TTL")
	s.Require().NotEmpty(token, "auth token should not be empty")

	// Verify the token
	verifiedPerms, err := client.Node.AuthVerify(ctx, token)
	s.Require().NoError(err, "should verify token with zero TTL")
	s.Assert().Equal(len(perms), len(verifiedPerms), "permissions should match")
}

// TestNodeLogLevelSet validates LogLevelSet API functionality
func (s *NodeTestSuite) TestNodeLogLevelSet_ValidLevels() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Test setting different log levels for valid loggers
	testCases := []struct {
		module string
		level  string
	}{
		{"blob", "error"}, // This logger should exist
	}

	for _, tc := range testCases {
		s.Run(tc.module+"_"+tc.level, func() {
			err := client.Node.LogLevelSet(ctx, tc.module, tc.level)
			s.Assert().NoError(err, "should set log level for %s to %s", tc.module, tc.level)
		})
	}
}

func (s *NodeTestSuite) TestNodeLogLevelSet_GlobalLevel() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Test setting global log level
	err := client.Node.LogLevelSet(ctx, "*", "info")
	s.Assert().NoError(err, "should set global log level")
}

func (s *NodeTestSuite) TestNodeLogLevelSet_InvalidLevel() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	// Test with invalid log level
	err := client.Node.LogLevelSet(ctx, "p2p", "invalid-level")
	s.Assert().Error(err, "should return error for invalid log level")
}

// TestNodeErrorHandling validates error handling scenarios
// Note: Network timeout error handling test removed as it's unreliable with current RPC client implementation and timing

// TestNodeCrossNodeConsistency validates consistency across different node types
func (s *NodeTestSuite) TestNodeCrossNodeConsistency() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test all node types
	nodes := []struct {
		name     string
		node     tastoratypes.DANode
		nodeType string
	}{
		{"full", s.framework.GetFullNodes()[0], "Full"},
		{"bridge", s.framework.GetBridgeNodes()[0], "Bridge"},
		{"light", s.framework.GetLightNodes()[0], "Light"},
	}

	s.Run("NodeInfo_Consistency", func() {
		for _, nodeData := range nodes {
			client := s.framework.GetNodeRPCClient(ctx, nodeData.node)

			info, err := client.Node.Info(ctx)
			s.Require().NoError(err, "should get info for %s node", nodeData.name)
			s.Assert().Equal(nodeData.nodeType, info.Type.String(), "%s node should have correct type", nodeData.name)
			s.Assert().NotEmpty(info.APIVersion, "%s node should have API version", nodeData.name)
		}
	})

	s.Run("Ready_Consistency", func() {
		for _, nodeData := range nodes {
			client := s.framework.GetNodeRPCClient(ctx, nodeData.node)

			ready, err := client.Node.Ready(ctx)
			s.Require().NoError(err, "should check ready for %s node", nodeData.name)
			s.Assert().True(ready, "%s node should be ready", nodeData.name)
		}
	})

	s.Run("Auth_Consistency", func() {
		for _, nodeData := range nodes {
			client := s.framework.GetNodeRPCClient(ctx, nodeData.node)

			// Create and verify token
			perms := []auth.Permission{"read"}
			token, err := client.Node.AuthNew(ctx, perms)
			s.Require().NoError(err, "should create token for %s node", nodeData.name)
			s.Require().NotEmpty(token, "token should not be empty for %s node", nodeData.name)

			verifiedPerms, err := client.Node.AuthVerify(ctx, token)
			s.Require().NoError(err, "should verify token for %s node", nodeData.name)
			s.Assert().Equal(len(perms), len(verifiedPerms), "permissions should match for %s node", nodeData.name)
		}
	})
}

// TestNodeResourceMonitoring validates resource usage and health monitoring
func (s *NodeTestSuite) TestNodeResourceMonitoring() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fullNode := s.framework.GetFullNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, fullNode)

	s.Run("NodeInfo_BuildVersion", func() {
		info, err := client.Node.Info(ctx)
		s.Require().NoError(err, "should get node info")

		// API version should be semantic version or "unknown"
		s.Assert().True(
			strings.Contains(info.APIVersion, ".") || info.APIVersion == "unknown",
			"API version should be semantic or unknown",
		)
	})

	s.Run("ReadyStatus_Stability", func() {
		// Check ready status multiple times to ensure stability
		for i := 0; i < 3; i++ {
			ready, err := client.Node.Ready(ctx)
			s.Require().NoError(err, "should check ready status (attempt %d)", i+1)
			s.Assert().True(ready, "node should remain ready (attempt %d)", i+1)

			time.Sleep(1 * time.Second)
		}
	})

	s.Run("AuthToken_Lifecycle", func() {
		// Test complete auth token lifecycle
		perms := []auth.Permission{"read", "admin"}

		// Create token
		token, err := client.Node.AuthNew(ctx, perms)
		s.Require().NoError(err, "should create token")

		// Verify immediately
		verifiedPerms, err := client.Node.AuthVerify(ctx, token)
		s.Require().NoError(err, "should verify token immediately")
		s.Assert().Equal(len(perms), len(verifiedPerms), "permissions should match")

		// Verify again after delay
		time.Sleep(2 * time.Second)
		verifiedPerms2, err := client.Node.AuthVerify(ctx, token)
		s.Require().NoError(err, "should verify token after delay")
		s.Assert().Equal(len(verifiedPerms), len(verifiedPerms2), "permissions should remain consistent")
	})
}
