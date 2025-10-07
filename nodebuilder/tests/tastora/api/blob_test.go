//go:build integration

package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/celestiaorg/go-square/v2/share"
	tastoradockertypes "github.com/celestiaorg/tastora/framework/docker"

	nodeblob "github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora"
	"github.com/celestiaorg/celestia-node/state"
)

// BlobJSONRPCTestSuite provides comprehensive testing of the Blob module APIs
// using pure JSON-RPC requests and response validation.
type BlobJSONRPCTestSuite struct {
	suite.Suite
	framework *tastora.Framework
}

func TestBlobJSONRPCTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Blob JSON-RPC integration tests in short mode")
	}
	suite.Run(t, &BlobJSONRPCTestSuite{})
}

func (s *BlobJSONRPCTestSuite) SetupSuite() {
	// Setup with bridge and light nodes for comprehensive JSON-RPC testing
	s.framework = tastora.NewFramework(s.T(), tastora.WithValidators(1), tastora.WithBridgeNodes(1), tastora.WithLightNodes(1))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))

	// Start the light node after network setup
	s.framework.NewLightNode(ctx)
}

// makeJSONRPCCall makes a raw JSON-RPC call to the node and returns the response
func (s *BlobJSONRPCTestSuite) makeJSONRPCCall(ctx context.Context, daNode *tastoradockertypes.DANode, method string, params interface{}) (map[string]interface{}, error) {
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

// validateJSONRPCResponse validates the basic JSON-RPC response structure
func (s *BlobJSONRPCTestSuite) validateJSONRPCResponse(response map[string]interface{}, nodeType, operation string) {
	// Validate response structure
	s.Require().Contains(response, "jsonrpc", "%s %s response should contain jsonrpc field", nodeType, operation)
	s.Require().Contains(response, "id", "%s %s response should contain id field", nodeType, operation)
	s.Require().Equal("2.0", response["jsonrpc"], "%s %s jsonrpc version should be 2.0", nodeType, operation)
	s.Require().Equal(float64(1), response["id"], "%s %s id should match request id", nodeType, operation)

	// Check for error in response
	if errorVal, hasError := response["error"]; hasError {
		s.T().Fatalf("%s %s JSON-RPC call failed with error: %v", nodeType, operation, errorVal)
	}
}

// TestBlobJSONRPCBridgeAndLightNodes tests JSON-RPC functionality on both bridge and light nodes
func (s *BlobJSONRPCTestSuite) TestBlobJSONRPCBridgeAndLightNodes() {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	bridgeClient := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	// Use existing light node for testing
	lightNode := s.framework.GetLightNodes()[0]
	lightClient := s.framework.GetNodeRPCClient(ctx, lightNode)

	// Wait for DHT to stabilize and light node to be ready
	time.Sleep(20 * time.Second)

	// Submit blob using bridge node (Go client)
	namespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x07}, 10))
	s.Require().NoError(err)

	data := []byte("Bridge and Light Node JSON-RPC test")
	nodeAddr, err := bridgeClient.State.AccountAddress(ctx)
	s.Require().NoError(err)

	libBlob, err := share.NewV1Blob(namespace, data, nodeAddr.Bytes())
	s.Require().NoError(err)

	nodeBlobs, err := nodeblob.ToNodeBlobs(libBlob)
	s.Require().NoError(err)

	// Submit blob using bridge node
	txConfig := state.NewTxConfig(state.WithGas(300_000), state.WithGasPrice(5000))
	height, err := bridgeClient.Blob.Submit(ctx, nodeBlobs, txConfig)
	s.Require().NoError(err, "Bridge node should be able to submit blob")
	s.Require().NotZero(height, "Bridge node submission should return valid height")

	// Wait for inclusion and light node sync
	_, err = bridgeClient.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err, "Should wait for blob inclusion")

	_, err = lightClient.Header.WaitForHeight(ctx, height)
	s.Require().NoError(err, "Light node should be synced to sufficient height")

	// Test JSON-RPC Get on both nodes
	s.Run("BridgeNodeJSONRPCGet", func() {
		params := []interface{}{
			height,
			base64.StdEncoding.EncodeToString(namespace.Bytes()),
			base64.StdEncoding.EncodeToString(nodeBlobs[0].Commitment),
		}

		response, err := s.makeJSONRPCCall(ctx, bridgeNode, "blob.Get", params)
		s.Require().NoError(err, "Bridge node JSON-RPC Get should succeed")

		// Validate response structure
		s.validateJSONRPCResponse(response, "Bridge node", "Get")

		// Validate result structure
		s.Require().Contains(response, "result", "Bridge node response should contain result field")
		result := response["result"].(map[string]interface{})

		// Validate blob fields
		s.Require().Contains(result, "namespace", "Bridge node result should contain namespace field")
		s.Require().Contains(result, "data", "Bridge node result should contain data field")
		s.Require().Contains(result, "share_version", "Bridge node result should contain share_version field")
		s.Require().Contains(result, "commitment", "Bridge node result should contain commitment field")
		s.Require().Contains(result, "index", "Bridge node result should contain index field")

		// Validate data content
		retrievedDataB64 := result["data"].(string)
		retrievedData, err := base64.StdEncoding.DecodeString(retrievedDataB64)
		s.Require().NoError(err)
		retrievedData = bytes.TrimRight(retrievedData, "\x00")
		s.Require().Equal(data, retrievedData, "Bridge node retrieved data should match original data")

		s.T().Logf("Bridge node JSON-RPC Get successful: data_length=%d", len(retrievedData))
	})

	s.Run("LightNodeJSONRPCGet", func() {
		params := []interface{}{
			height,
			base64.StdEncoding.EncodeToString(namespace.Bytes()),
			base64.StdEncoding.EncodeToString(nodeBlobs[0].Commitment),
		}

		response, err := s.makeJSONRPCCall(ctx, lightNode, "blob.Get", params)
		s.Require().NoError(err, "Light node JSON-RPC Get should succeed")

		// Validate response structure
		s.validateJSONRPCResponse(response, "Light node", "Get")

		// Validate result structure
		s.Require().Contains(response, "result", "Light node response should contain result field")
		result := response["result"].(map[string]interface{})

		// Validate blob fields
		s.Require().Contains(result, "namespace", "Light node result should contain namespace field")
		s.Require().Contains(result, "data", "Light node result should contain data field")
		s.Require().Contains(result, "share_version", "Light node result should contain share_version field")
		s.Require().Contains(result, "commitment", "Light node result should contain commitment field")
		s.Require().Contains(result, "index", "Light node result should contain index field")

		// Validate data content
		retrievedDataB64 := result["data"].(string)
		retrievedData, err := base64.StdEncoding.DecodeString(retrievedDataB64)
		s.Require().NoError(err)
		retrievedData = bytes.TrimRight(retrievedData, "\x00")
		s.Require().Equal(data, retrievedData, "Light node retrieved data should match original data")

		s.T().Logf("Light node JSON-RPC Get successful: data_length=%d", len(retrievedData))
	})

	// Test JSON-RPC GetAll on both nodes
	s.Run("BridgeNodeJSONRPCGetAll", func() {
		params := []interface{}{
			height,
			[]interface{}{base64.StdEncoding.EncodeToString(namespace.Bytes())},
		}

		response, err := s.makeJSONRPCCall(ctx, bridgeNode, "blob.GetAll", params)
		s.Require().NoError(err, "Bridge node JSON-RPC GetAll should succeed")

		// Validate response structure
		s.validateJSONRPCResponse(response, "Bridge node", "GetAll")

		// Validate result structure
		s.Require().Contains(response, "result", "Bridge node GetAll response should contain result field")
		result := response["result"].([]interface{})

		// Should have at least 1 blob
		s.Require().GreaterOrEqual(len(result), 1, "Bridge node GetAll should return at least 1 blob")

		s.T().Logf("Bridge node JSON-RPC GetAll successful: blob_count=%d", len(result))
	})

	s.Run("LightNodeJSONRPCGetAll", func() {
		params := []interface{}{
			height,
			[]interface{}{base64.StdEncoding.EncodeToString(namespace.Bytes())},
		}

		response, err := s.makeJSONRPCCall(ctx, lightNode, "blob.GetAll", params)
		s.Require().NoError(err, "Light node JSON-RPC GetAll should succeed")

		// Validate response structure
		s.validateJSONRPCResponse(response, "Light node", "GetAll")

		// Validate result structure
		s.Require().Contains(response, "result", "Light node GetAll response should contain result field")
		result := response["result"].([]interface{})

		// Should have at least 1 blob
		s.Require().GreaterOrEqual(len(result), 1, "Light node GetAll should return at least 1 blob")

		s.T().Logf("Light node JSON-RPC GetAll successful: blob_count=%d", len(result))
	})

	// Test JSON-RPC GetProof on both nodes
	s.Run("BridgeNodeJSONRPCGetProof", func() {
		params := []interface{}{
			height,
			base64.StdEncoding.EncodeToString(namespace.Bytes()),
			base64.StdEncoding.EncodeToString(nodeBlobs[0].Commitment),
		}

		response, err := s.makeJSONRPCCall(ctx, bridgeNode, "blob.GetProof", params)
		s.Require().NoError(err, "Bridge node JSON-RPC GetProof should succeed")

		// Validate response structure
		s.validateJSONRPCResponse(response, "Bridge node", "GetProof")

		// Validate result structure
		s.Require().Contains(response, "result", "Bridge node GetProof response should contain result field")
		result := response["result"].([]interface{})

		// Proof should be a non-empty array
		s.Require().NotEmpty(result, "Bridge node GetProof should not be empty")

		s.T().Logf("Bridge node JSON-RPC GetProof successful: proof_length=%d", len(result))
	})

	s.Run("LightNodeJSONRPCGetProof", func() {
		params := []interface{}{
			height,
			base64.StdEncoding.EncodeToString(namespace.Bytes()),
			base64.StdEncoding.EncodeToString(nodeBlobs[0].Commitment),
		}

		response, err := s.makeJSONRPCCall(ctx, lightNode, "blob.GetProof", params)
		s.Require().NoError(err, "Light node JSON-RPC GetProof should succeed")

		// Validate response structure
		s.validateJSONRPCResponse(response, "Light node", "GetProof")

		// Validate result structure
		s.Require().Contains(response, "result", "Light node GetProof response should contain result field")
		result := response["result"].([]interface{})

		// Proof should be a non-empty array
		s.Require().NotEmpty(result, "Light node GetProof should not be empty")

		s.T().Logf("Light node JSON-RPC GetProof successful: proof_length=%d", len(result))
	})

	// Test JSON-RPC Submit on both nodes using correct TxConfig format
	s.Run("BridgeNodeJSONRPCSubmit", func() {
		submitNamespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x08}, 10))
		s.Require().NoError(err)

		submitData := []byte("Bridge node JSON-RPC Submit test")

		// Prepare JSON-RPC request parameters
		blobParam := map[string]interface{}{
			"namespace":     base64.StdEncoding.EncodeToString(submitNamespace.Bytes()),
			"data":          base64.StdEncoding.EncodeToString(submitData),
			"share_version": 0,
		}

		// Use correct TxConfig JSON format based on jsonTxConfig struct
		txConfigParam := map[string]interface{}{
			"gas":                 300000,
			"gas_price":           5000.0,
			"is_gas_price_set":    true,
			"max_gas_price":       0.1,
			"tx_priority":         2, // TxPriorityMedium
			"key_name":            "",
			"signer_address":      "",
			"fee_granter_address": "",
		}

		params := []interface{}{[]interface{}{blobParam}, txConfigParam}

		response, err := s.makeJSONRPCCall(ctx, bridgeNode, "blob.Submit", params)
		s.Require().NoError(err, "Bridge node JSON-RPC Submit should succeed")

		// Validate response structure
		s.validateJSONRPCResponse(response, "Bridge node", "Submit")

		// If no error, validate result
		if _, hasError := response["error"]; !hasError {
			s.Require().Contains(response, "result", "Bridge node Submit response should contain result field")
			result := response["result"].(float64)
			s.Require().NotZero(result, "Bridge node Submit should return valid height")
			s.T().Logf("Bridge node JSON-RPC Submit successful: height=%v", result)
		}
	})

	s.Run("LightNodeJSONRPCSubmit", func() {
		submitNamespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x09}, 10))
		s.Require().NoError(err)

		submitData := []byte("Light node JSON-RPC Submit test")

		// Prepare JSON-RPC request parameters
		blobParam := map[string]interface{}{
			"namespace":     base64.StdEncoding.EncodeToString(submitNamespace.Bytes()),
			"data":          base64.StdEncoding.EncodeToString(submitData),
			"share_version": 0,
		}

		// Use correct TxConfig JSON format based on jsonTxConfig struct
		txConfigParam := map[string]interface{}{
			"gas":                 300000,
			"gas_price":           5000.0,
			"is_gas_price_set":    true,
			"max_gas_price":       0.1,
			"tx_priority":         2, // TxPriorityMedium
			"key_name":            "",
			"signer_address":      "",
			"fee_granter_address": "",
		}

		params := []interface{}{[]interface{}{blobParam}, txConfigParam}

		response, err := s.makeJSONRPCCall(ctx, lightNode, "blob.Submit", params)
		s.Require().NoError(err, "Light node JSON-RPC Submit should succeed")

		// Validate response structure
		s.validateJSONRPCResponse(response, "Light node", "Submit")

		// If no error, validate result
		if _, hasError := response["error"]; !hasError {
			s.Require().Contains(response, "result", "Light node Submit response should contain result field")
			result := response["result"].(float64)
			s.Require().NotZero(result, "Light node Submit should return valid height")
			s.T().Logf("Light node JSON-RPC Submit successful: height=%v", result)
		}
	})

	// Test JSON-RPC Included on both nodes
	s.Run("BridgeNodeJSONRPCIncluded", func() {
		// Get proof first (using the blob from earlier in the test)
		proof, err := bridgeClient.Blob.GetProof(ctx, height, namespace, nodeBlobs[0].Commitment)
		s.Require().NoError(err, "Bridge node should be able to get proof")

		// Serialize proof as JSON array of proof objects
		proofJSON, err := json.Marshal(proof)
		s.Require().NoError(err)

		var proofArray []interface{}
		err = json.Unmarshal(proofJSON, &proofArray)
		s.Require().NoError(err)

		params := []interface{}{
			height,
			base64.StdEncoding.EncodeToString(namespace.Bytes()),
			proofArray,
			base64.StdEncoding.EncodeToString(nodeBlobs[0].Commitment),
		}

		response, err := s.makeJSONRPCCall(ctx, bridgeNode, "blob.Included", params)
		s.Require().NoError(err, "Bridge node JSON-RPC Included should succeed")

		// Validate response structure
		s.validateJSONRPCResponse(response, "Bridge node", "Included")
		result := response["result"].(bool)
		s.Require().True(result, "Bridge node Included should return true for valid proof")

		s.T().Logf("Bridge node JSON-RPC Included successful: included=%v", result)
	})

	s.Run("LightNodeJSONRPCIncluded", func() {
		// Get proof first (using the blob from earlier in the test)
		proof, err := lightClient.Blob.GetProof(ctx, height, namespace, nodeBlobs[0].Commitment)
		s.Require().NoError(err, "Light node should be able to get proof")

		// Serialize proof as JSON array of proof objects
		proofJSON, err := json.Marshal(proof)
		s.Require().NoError(err)

		var proofArray []interface{}
		err = json.Unmarshal(proofJSON, &proofArray)
		s.Require().NoError(err)

		params := []interface{}{
			height,
			base64.StdEncoding.EncodeToString(namespace.Bytes()),
			proofArray,
			base64.StdEncoding.EncodeToString(nodeBlobs[0].Commitment),
		}

		response, err := s.makeJSONRPCCall(ctx, lightNode, "blob.Included", params)
		s.Require().NoError(err, "Light node JSON-RPC Included should succeed")

		// Validate response structure
		s.validateJSONRPCResponse(response, "Light node", "Included")
		result := response["result"].(bool)
		s.Require().True(result, "Light node Included should return true for valid proof")

		s.T().Logf("Light node JSON-RPC Included successful: included=%v", result)
	})

	s.T().Logf("Successfully tested all blob JSON-RPC APIs on both bridge and light nodes")
}
