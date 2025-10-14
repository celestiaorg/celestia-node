//go:build integration

package tastora

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/celestiaorg/go-square/v2/share"

	nodeblob "github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/state"
)

// BlobMetricsOutputTestSuite tests blob submission metrics and displays the actual metrics output
type BlobMetricsOutputTestSuite struct {
	suite.Suite
	framework      *Framework
	submittedBlobs []*nodeblob.Blob
}

func TestBlobMetricsOutputTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping blob metrics integration tests in short mode")
	}
	suite.Run(t, &BlobMetricsOutputTestSuite{})
}

func (s *BlobMetricsOutputTestSuite) SetupSuite() {
	s.framework = NewFramework(s.T(), WithValidators(1))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
}

func (s *BlobMetricsOutputTestSuite) TearDownSuite() {
	if s.framework != nil {
		// Framework cleanup is handled automatically by the test framework
		s.T().Log("Test suite teardown completed")
	}
}

// TestBlobMetricsOutput tests blob submission and displays the actual metrics output
func (s *BlobMetricsOutputTestSuite) TestBlobMetricsOutput() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, bridgeNode)

	s.T().Log("Starting blob metrics output test...")
	s.T().Log("Bridge node metrics endpoint: http://localhost:8890/metrics")

	// Create test namespace
	namespace, err := share.NewV0Namespace(bytes.Repeat([]byte{0x01}, 10))
	s.Require().NoError(err)

	// Perform multiple blob submissions to accumulate metrics
	submissionCount := 3
	for i := 0; i < submissionCount; i++ {
		s.T().Logf("Performing blob submission %d/%d", i+1, submissionCount)

		// Create test blob data
		blobData := []byte(fmt.Sprintf("test-blob-data-%d", i))
		blob, err := nodeblob.NewBlobV0(namespace, blobData)
		s.Require().NoError(err)

		// Submit blob
		height, err := client.Blob.Submit(ctx, []*nodeblob.Blob{blob}, state.NewTxConfig(
			state.WithGas(100000),
			state.WithGasPrice(0.025), // Set minimum gas price
		))
		s.Require().NoError(err, "blob submission %d failed", i+1)
		s.Require().Greater(height, uint64(0), "invalid height returned for submission %d", i+1)

		// Store the submitted blob for later retrieval testing
		s.submittedBlobs = append(s.submittedBlobs, blob)

		s.T().Logf("Blob submission %d successful at height %d", i+1, height)

		// Wait a bit between submissions to spread metrics over time
		time.Sleep(2 * time.Second)
	}

	// Perform some blob retrieval operations to generate retrieval metrics
	s.T().Log("Performing blob retrieval operations...")
	for i := 0; i < 2; i++ {
		// Get the latest header to retrieve blobs from
		err := client.Header.SyncWait(ctx)
		s.Require().NoError(err)

		// Get the latest header
		header, err := client.Header.NetworkHead(ctx)
		s.Require().NoError(err)

		// Try to retrieve a specific blob using Get method (which records metrics)
		if len(s.submittedBlobs) > 0 {
			blob, err := client.Blob.Get(ctx, header.Height(), namespace, s.submittedBlobs[0].Commitment)
			if err != nil {
				s.T().Logf("Blob retrieval attempt %d: %v (expected for some heights)", i+1, err)
			} else {
				s.T().Logf("Blob retrieval attempt %d: found blob with commitment %x", i+1, blob.Commitment)
			}
		}

		time.Sleep(1 * time.Second)
	}

	// Test blob proof generation to trigger proof metrics
	s.T().Log("Performing blob proof operations...")
	for i := 0; i < 2; i++ {
		if len(s.submittedBlobs) > 0 {
			// Get the latest header
			header, err := client.Header.NetworkHead(ctx)
			s.Require().NoError(err)

			// Try to get proof for the first submitted blob
			proof, err := client.Blob.GetProof(ctx, header.Height(), namespace, s.submittedBlobs[0].Commitment)
			if err != nil {
				s.T().Logf("Blob proof attempt %d: %v (expected for some heights)", i+1, err)
			} else {
				s.T().Logf("Blob proof attempt %d: generated proof with %d nmt proofs", i+1, len(*proof))
			}
		}
		time.Sleep(1 * time.Second)
	}

	// Wait for all metrics to be collected
	s.T().Log("Final metrics collection wait...")
	time.Sleep(3 * time.Second)

	// Try to fetch metrics from the bridge node using docker exec
	s.T().Log("Fetching metrics from bridge node...")

	// Get the container name from the bridge node - remove the test suite name part
	testName := s.T().Name()
	containerName := fmt.Sprintf("bridge-0-%s", testName)
	s.T().Logf("Container name: %s", containerName)

	// Also try to list running containers to debug
	cmd := exec.Command("docker", "ps", "--format", "table {{.Names}}")
	output, err := cmd.Output()
	if err == nil {
		s.T().Logf("Running containers:\n%s", string(output))
	}

	// Execute curl command inside the bridge node container to get metrics
	metricsCmd := exec.Command("docker", "exec", containerName, "curl", "-s", "http://localhost:8890/metrics")
	metricsOutput, err := metricsCmd.Output()
	if err != nil {
		s.T().Logf("Failed to fetch metrics from container: %v", err)
		return
	}

	metricsContent := metricsOutput

	// Extract and display blob-related metrics
	s.T().Log("🔍 BLOB METRICS OUTPUT:")
	s.T().Log("======================")

	lines := bytes.Split(metricsContent, []byte("\n"))
	blobMetrics := []string{}

	for _, line := range lines {
		lineStr := string(line)
		if bytes.Contains(line, []byte("blob_")) ||
			bytes.Contains(line, []byte("state_pfb_")) ||
			bytes.Contains(line, []byte("state_gas_")) {
			blobMetrics = append(blobMetrics, lineStr)
		}
	}

	if len(blobMetrics) == 0 {
		s.T().Log("❌ No blob metrics found!")
		s.T().Log("📝 Full metrics content (first 2000 chars):")
		if len(metricsContent) > 2000 {
			s.T().Log(string(metricsContent[:2000]) + "...")
		} else {
			s.T().Log(string(metricsContent))
		}
	} else {
		s.T().Logf("✅ Found %d blob-related metrics:\n", len(blobMetrics))
		for _, metric := range blobMetrics {
			if metric != "" {
				s.T().Log(metric)
			}
		}
	}

	s.T().Log("✅ Blob metrics output test completed!")
	s.T().Log("📊 Metrics endpoint: http://localhost:8890/metrics")
}
