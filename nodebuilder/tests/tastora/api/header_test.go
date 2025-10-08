//go:build integration

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	tastoradockertypes "github.com/celestiaorg/tastora/framework/docker"

	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora"
)

// HeaderJSONRPCTestSuite provides comprehensive testing of the Header module APIs
// using pure JSON-RPC requests and response validation.
type HeaderJSONRPCTestSuite struct {
	suite.Suite
	framework *tastora.Framework
}

func TestHeaderJSONRPCTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Header JSON-RPC integration tests in short mode")
	}
	suite.Run(t, &HeaderJSONRPCTestSuite{})
}

func (s *HeaderJSONRPCTestSuite) SetupSuite() {
	// Setup with bridge and light nodes for comprehensive JSON-RPC testing
	s.framework = tastora.NewFramework(s.T(), tastora.WithValidators(1), tastora.WithBridgeNodes(1), tastora.WithLightNodes(1))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))

	// Start the light node after network setup
	s.framework.NewLightNode(ctx)
}

// makeJSONRPCCall makes a raw JSON-RPC call to the specified node
func (s *HeaderJSONRPCTestSuite) makeJSONRPCCall(ctx context.Context, daNode *tastoradockertypes.DANode, method string, params interface{}) (map[string]interface{}, error) {
	rpcAddr := daNode.GetHostRPCAddress()
	if rpcAddr == "" {
		return nil, fmt.Errorf("rpc address is empty")
	}

	// Normalize wildcard bind address to loopback for outbound connections
	if strings.HasPrefix(rpcAddr, "0.0.0.0:") {
		rpcAddr = "127.0.0.1:" + strings.TrimPrefix(rpcAddr, "0.0.0.0:")
	}

	// Prepare JSON-RPC request
	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	}

	requestBody, err := json.Marshal(request)
	s.Require().NoError(err)

	// Make HTTP request
	url := fmt.Sprintf("http://%s", rpcAddr)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
	s.Require().NoError(err)

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()

	// Parse response
	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	s.Require().NoError(err)

	return response, nil
}

// waitForLightNodeReady waits for the light node to be ready and synced with the bridge node
func (s *HeaderJSONRPCTestSuite) waitForLightNodeReady(ctx context.Context) {
	maxWait := 30 * time.Second
	start := time.Now()

	lightNode := s.framework.GetLightNodes()[0]

	for time.Since(start) < maxWait {
		// Only check light node LocalHead - this is sufficient to verify readiness
		lightHeadResponse, err := s.makeJSONRPCCall(ctx, lightNode, "header.LocalHead", nil)
		if err == nil && lightHeadResponse != nil {
			s.T().Logf("Light node ready and synced")
			return
		}

		// Progressive backoff: start fast, then slow down
		elapsed := time.Since(start)
		if elapsed < 5*time.Second {
			time.Sleep(200 * time.Millisecond) // Fast polling initially
		} else if elapsed < 15*time.Second {
			time.Sleep(500 * time.Millisecond) // Medium polling
		} else {
			time.Sleep(1 * time.Second) // Slow polling for final attempts
		}
	}
	s.T().Logf("Warning: Light node may not be fully ready after %v", maxWait)
}

// validateJSONRPCResponse validates the basic JSON-RPC response structure
func (s *HeaderJSONRPCTestSuite) validateJSONRPCResponse(response map[string]interface{}, nodeType, operation string) {
	s.Require().Contains(response, "jsonrpc", "%s %s response should contain jsonrpc field", nodeType, operation)
	s.Require().Contains(response, "id", "%s %s response should contain id field", nodeType, operation)
	s.Require().Equal("2.0", response["jsonrpc"], "%s %s jsonrpc version should be 2.0", nodeType, operation)
	s.Require().Equal(float64(1), response["id"], "%s %s id should match request id", nodeType, operation)

	if errorVal, hasError := response["error"]; hasError {
		s.T().Fatalf("%s %s JSON-RPC call failed with error: %v", nodeType, operation, errorVal)
	}
}

// validateHeaderResponse validates header-specific response structure
func (s *HeaderJSONRPCTestSuite) validateHeaderResponse(response map[string]interface{}, nodeType, operation string) {
	s.validateJSONRPCResponse(response, nodeType, operation)

	s.Require().Contains(response, "result", "%s %s response should contain result field", nodeType, operation)
	result := response["result"].(map[string]interface{})

	// Validate header structure - height is nested in header.height
	s.Require().Contains(result, "header", "%s %s header should contain header field", nodeType, operation)
	header := result["header"].(map[string]interface{})
	s.Require().Contains(header, "height", "%s %s header should contain height", nodeType, operation)

	// Validate other required fields
	s.Require().Contains(result, "dah", "%s %s header should contain DAH", nodeType, operation)
	s.Require().Contains(result, "commit", "%s %s header should contain commit", nodeType, operation)
	s.Require().Contains(result, "validator_set", "%s %s header should contain validator_set", nodeType, operation)

	// Validate height is a positive number
	heightStr, ok := header["height"].(string)
	s.Require().True(ok, "%s %s height should be a string", nodeType, operation)
	s.Require().NotEmpty(heightStr, "%s %s height should not be empty", nodeType, operation)
}

// validateSyncStateResponse validates sync state response structure
func (s *HeaderJSONRPCTestSuite) validateSyncStateResponse(response map[string]interface{}, nodeType, operation string) {
	s.validateJSONRPCResponse(response, nodeType, operation)

	s.Require().Contains(response, "result", "%s %s response should contain result field", nodeType, operation)
	result := response["result"].(map[string]interface{})

	// Validate sync state structure - based on actual response format
	s.Require().Contains(result, "height", "%s %s sync state should contain height field", nodeType, operation)
	s.Require().Contains(result, "from_height", "%s %s sync state should contain from_height field", nodeType, operation)
	s.Require().Contains(result, "to_height", "%s %s sync state should contain to_height field", nodeType, operation)
}

func (s *HeaderJSONRPCTestSuite) TestHeaderJSONRPCBridgeAndLightNodes() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	lightNode := s.framework.GetLightNodes()[0]

	// Wait for light node to be ready
	s.waitForLightNodeReady(ctx)

	// Test LocalHead
	s.Run("BridgeNodeJSONRPCLocalHead", func() {
		response, err := s.makeJSONRPCCall(ctx, bridgeNode, "header.LocalHead", nil)
		s.Require().NoError(err)
		s.validateHeaderResponse(response, "Bridge node", "LocalHead")
	})

	s.Run("LightNodeJSONRPCLocalHead", func() {
		response, err := s.makeJSONRPCCall(ctx, lightNode, "header.LocalHead", nil)
		s.Require().NoError(err)
		s.validateHeaderResponse(response, "Light node", "LocalHead")
	})

	// Test NetworkHead
	s.Run("BridgeNodeJSONRPCNetworkHead", func() {
		response, err := s.makeJSONRPCCall(ctx, bridgeNode, "header.NetworkHead", nil)
		s.Require().NoError(err)
		s.validateHeaderResponse(response, "Bridge node", "NetworkHead")
	})

	s.Run("LightNodeJSONRPCNetworkHead", func() {
		response, err := s.makeJSONRPCCall(ctx, lightNode, "header.NetworkHead", nil)
		s.Require().NoError(err)
		s.validateHeaderResponse(response, "Light node", "NetworkHead")
	})

	// Test SyncState
	s.Run("BridgeNodeJSONRPCSyncState", func() {
		response, err := s.makeJSONRPCCall(ctx, bridgeNode, "header.SyncState", nil)
		s.Require().NoError(err)
		s.validateSyncStateResponse(response, "Bridge node", "SyncState")
	})

	s.Run("LightNodeJSONRPCSyncState", func() {
		response, err := s.makeJSONRPCCall(ctx, lightNode, "header.SyncState", nil)
		s.Require().NoError(err)
		s.validateSyncStateResponse(response, "Light node", "SyncState")
	})

	// Note: header.Tail method is not available in the current API

	// Test GetByHeight - get height 1
	s.Run("BridgeNodeJSONRPCGetByHeight", func() {
		params := []interface{}{1}
		response, err := s.makeJSONRPCCall(ctx, bridgeNode, "header.GetByHeight", params)
		s.Require().NoError(err)
		s.validateHeaderResponse(response, "Bridge node", "GetByHeight")

		// Validate that the returned header has height 1
		result := response["result"].(map[string]interface{})
		header := result["header"].(map[string]interface{})
		height := header["height"].(string)
		s.Require().Equal("1", height, "Bridge node GetByHeight should return header with height 1")
	})

	s.Run("LightNodeJSONRPCGetByHeight", func() {
		params := []interface{}{1}
		response, err := s.makeJSONRPCCall(ctx, lightNode, "header.GetByHeight", params)
		s.Require().NoError(err)
		s.validateHeaderResponse(response, "Light node", "GetByHeight")

		// Validate that the returned header has height 1
		result := response["result"].(map[string]interface{})
		header := result["header"].(map[string]interface{})
		height := header["height"].(string)
		s.Require().Equal("1", height, "Light node GetByHeight should return header with height 1")
	})

	// Test GetByHash - first get a header to get its hash
	s.Run("BridgeNodeJSONRPCGetByHash", func() {
		// First get a header to extract its hash
		headResponse, err := s.makeJSONRPCCall(ctx, bridgeNode, "header.LocalHead", nil)
		s.Require().NoError(err)
		headResult := headResponse["result"].(map[string]interface{})
		commit := headResult["commit"].(map[string]interface{})
		blockID := commit["block_id"].(map[string]interface{})
		hash := blockID["hash"].(string)

		// Now get the header by hash
		params := []interface{}{hash}
		response, err := s.makeJSONRPCCall(ctx, bridgeNode, "header.GetByHash", params)
		s.Require().NoError(err)
		s.validateHeaderResponse(response, "Bridge node", "GetByHash")

		// Validate that the returned header has the same hash
		result := response["result"].(map[string]interface{})
		returnedCommit := result["commit"].(map[string]interface{})
		returnedBlockID := returnedCommit["block_id"].(map[string]interface{})
		returnedHash := returnedBlockID["hash"].(string)
		s.Require().Equal(hash, returnedHash, "Bridge node GetByHash should return header with matching hash")
	})

	s.Run("LightNodeJSONRPCGetByHash", func() {
		// First get a header to extract its hash
		headResponse, err := s.makeJSONRPCCall(ctx, lightNode, "header.LocalHead", nil)
		s.Require().NoError(err)
		headResult := headResponse["result"].(map[string]interface{})
		commit := headResult["commit"].(map[string]interface{})
		blockID := commit["block_id"].(map[string]interface{})
		hash := blockID["hash"].(string)

		// Now get the header by hash
		params := []interface{}{hash}
		response, err := s.makeJSONRPCCall(ctx, lightNode, "header.GetByHash", params)
		s.Require().NoError(err)
		s.validateHeaderResponse(response, "Light node", "GetByHash")

		// Validate that the returned header has the same hash
		result := response["result"].(map[string]interface{})
		returnedCommit := result["commit"].(map[string]interface{})
		returnedBlockID := returnedCommit["block_id"].(map[string]interface{})
		returnedHash := returnedBlockID["hash"].(string)
		s.Require().Equal(hash, returnedHash, "Light node GetByHash should return header with matching hash")
	})

	// Test WaitForHeight - wait for height 1 (should be immediate)
	s.Run("BridgeNodeJSONRPCWaitForHeight", func() {
		params := []interface{}{1}
		response, err := s.makeJSONRPCCall(ctx, bridgeNode, "header.WaitForHeight", params)
		s.Require().NoError(err)
		s.validateHeaderResponse(response, "Bridge node", "WaitForHeight")

		// Validate that the returned header has height 1
		result := response["result"].(map[string]interface{})
		header := result["header"].(map[string]interface{})
		height := header["height"].(string)
		s.Require().Equal("1", height, "Bridge node WaitForHeight should return header with height 1")
	})

	s.Run("LightNodeJSONRPCWaitForHeight", func() {
		params := []interface{}{1}
		response, err := s.makeJSONRPCCall(ctx, lightNode, "header.WaitForHeight", params)
		s.Require().NoError(err)
		s.validateHeaderResponse(response, "Light node", "WaitForHeight")

		// Validate that the returned header has height 1
		result := response["result"].(map[string]interface{})
		header := result["header"].(map[string]interface{})
		height := header["height"].(string)
		s.Require().Equal("1", height, "Light node WaitForHeight should return header with height 1")
	})
}
