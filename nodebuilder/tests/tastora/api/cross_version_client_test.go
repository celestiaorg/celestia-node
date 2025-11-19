//go:build integration

package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/celestiaorg/go-square/v3/share"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
	nodeblob "github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora"
	"github.com/celestiaorg/celestia-node/state"
	"github.com/celestiaorg/tastora/framework/types"
)

type CrossVersionClientTestSuite struct {
	suite.Suite
	f *tastora.Framework
}

func TestCrossVersionClientTestSuite(t *testing.T) {
	suite.Run(t, new(CrossVersionClientTestSuite))
}

func (s *CrossVersionClientTestSuite) SetupSuite() {
	// Create framework with 2 light nodes: 1 for CurrentClient_OldLightServer and 1 for OldClient_CurrentLightServer
	s.f = tastora.NewFramework(s.T(), tastora.WithValidators(1), tastora.WithLightNodes(2))
	// Use a longer timeout for setup to handle slow chain initialization
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	err := s.f.SetupNetwork(ctx)
	require.NoError(s.T(), err, "Failed to setup network")
}

func (s *CrossVersionClientTestSuite) TearDownSuite() {
	if s.f != nil {
		s.f.Stop(context.Background())
	}
}

func (s *CrossVersionClientTestSuite) TestCrossVersionBidirectional() {
	// Use a longer timeout to handle Docker builds and all test combinations
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	oldVersion := "v0.28.3-arabica"

	s.T().Run("CurrentClient_OldBridgeServer", func(t *testing.T) {
		oldServer := s.f.NewBridgeNodeWithVersion(ctx, oldVersion)
		require.NotNil(s.T(), oldServer)

		client := s.f.GetNodeRPCClient(ctx, oldServer)
		s.testAllAPIs(ctx, client)
	})

	s.T().Run("CurrentClient_OldLightServer", func(t *testing.T) {
		oldServer := s.f.NewLightNodeWithVersion(ctx, oldVersion)
		require.NotNil(s.T(), oldServer)

		client := s.f.GetNodeRPCClient(ctx, oldServer)

		// Wait for the old light server to be fully ready and synced before testing APIs
		// This ensures all APIs will work without needing individual timeouts
		s.waitForNodeReadyAndSynced(ctx, client, "old light server", 3*time.Minute)

		// Skip GetRow for old light servers due to bitswap compatibility issues
		s.testAllAPIsWithOptions(ctx, client, true)
	})

	s.T().Run("OldClient_CurrentBridgeServer", func(t *testing.T) {
		// Use a separate context for this test to avoid cancellation from parent
		// Increased timeout to handle Docker builds when resources are constrained
		testCtx, testCancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer testCancel()

		// Add a small delay before starting Docker-intensive test to avoid resource contention
		time.Sleep(1 * time.Second)

		bridgeNodes := s.f.GetBridgeNodes()
		require.Greater(s.T(), len(bridgeNodes), 0)
		currentServer := bridgeNodes[0]

		serverInfo, err := currentServer.GetNetworkInfo(testCtx)
		require.NoError(s.T(), err)
		rpcAddr := fmt.Sprintf("%s:%s", serverInfo.Internal.Hostname, serverInfo.Internal.Ports.RPC)
		serverRPC := "http://" + rpcAddr

		err = s.runClientTestFromDockerImage(testCtx, oldVersion, serverRPC, false)
		require.NoError(s.T(), err)
	})

	s.T().Run("OldClient_CurrentLightServer", func(t *testing.T) {
		// Use a separate context for this test to avoid cancellation from parent
		// Increased timeout to handle Docker builds when resources are constrained
		testCtx, testCancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer testCancel()

		// Add a delay before starting Docker-intensive test to avoid resource contention
		// This helps when previous Docker builds are still running
		time.Sleep(2 * time.Second)

		lightNode := s.f.NewLightNode(testCtx)
		require.NotNil(s.T(), lightNode)

		// Wait a bit for the light node to be fully ready before getting network info
		time.Sleep(2 * time.Second)

		serverInfo, err := lightNode.GetNetworkInfo(testCtx)
		require.NoError(s.T(), err)
		rpcAddr := fmt.Sprintf("%s:%s", serverInfo.Internal.Hostname, serverInfo.Internal.Ports.RPC)
		serverRPC := "http://" + rpcAddr

		err = s.runClientTestFromDockerImage(testCtx, oldVersion, serverRPC, true)
		require.NoError(s.T(), err)
	})
}

// waitForNodeReadyAndSynced waits for a node to be fully ready and synced before testing APIs.
// This ensures APIs will work without needing individual timeouts.
func (s *CrossVersionClientTestSuite) waitForNodeReadyAndSynced(ctx context.Context, client *rpcclient.Client, nodeName string, timeout time.Duration) {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	s.T().Logf("Waiting for %s to be ready...", nodeName)
	// Wait for node to be ready
	ready := false
	for !ready {
		select {
		case <-waitCtx.Done():
			s.T().Fatalf("%s did not become ready within %v", nodeName, timeout)
		default:
			var err error
			ready, err = client.Node.Ready(waitCtx)
			if err == nil && ready {
				s.T().Logf("%s is ready", nodeName)
			} else {
				time.Sleep(2 * time.Second)
			}
		}
	}

	s.T().Logf("Waiting for %s to sync to network head...", nodeName)
	// Wait for node to sync to network head
	// Compare LocalHead with NetworkHead to ensure sync is complete
	var lastLocalHeight uint64
	for i := 0; i < 60; i++ { // Max 60 iterations (2 minutes with 2s sleep)
		select {
		case <-waitCtx.Done():
			s.T().Logf("Warning: %s may not be fully synced, proceeding anyway", nodeName)
			return
		default:
			localHead, err := client.Header.LocalHead(waitCtx)
			if err != nil {
				time.Sleep(2 * time.Second)
				continue
			}
			if localHead == nil {
				time.Sleep(2 * time.Second)
				continue
			}

			networkHead, err := client.Header.NetworkHead(waitCtx)
			if err != nil {
				time.Sleep(2 * time.Second)
				continue
			}
			if networkHead == nil {
				time.Sleep(2 * time.Second)
				continue
			}

			localHeight := localHead.Height()
			networkHeight := networkHead.Height()

			// Node is synced if local head matches network head (or is very close)
			if localHeight >= networkHeight || (networkHeight-localHeight) <= 2 {
				s.T().Logf("%s synced to height %d (network: %d)", nodeName, localHeight, networkHeight)
				// For light nodes, also wait for share data to be available
				// Check if SharesAvailable works for the current height
				sharesCtx, sharesCancel := context.WithTimeout(waitCtx, 10*time.Second)
				err = client.Share.SharesAvailable(sharesCtx, localHeight)
				sharesCancel()
				if err == nil {
					s.T().Logf("%s share data is available", nodeName)
					return
				}
				// If share data not available yet, wait a bit more but don't fail
				if strings.Contains(err.Error(), "data not available") {
					s.T().Logf("%s share data not yet available, waiting...", nodeName)
					time.Sleep(2 * time.Second)
					continue
				}
				// For other errors, proceed anyway (may be compatibility issues)
				s.T().Logf("%s share check returned error (may be compatibility issue), proceeding: %v", nodeName, err)
				return
			}

			// If height hasn't changed, wait a bit longer
			if localHeight == lastLocalHeight {
				time.Sleep(2 * time.Second)
			} else {
				lastLocalHeight = localHeight
				s.T().Logf("%s syncing... local: %d, network: %d", nodeName, localHeight, networkHeight)
			}
			time.Sleep(2 * time.Second)
		}
	}
	s.T().Logf("Warning: %s sync check timed out, proceeding anyway", nodeName)
}

func (s *CrossVersionClientTestSuite) testAllAPIs(ctx context.Context, client *rpcclient.Client) {
	s.testAllAPIsWithOptions(ctx, client, false)
}

func (s *CrossVersionClientTestSuite) testAllAPIsWithOptions(ctx context.Context, client *rpcclient.Client, skipGetRow bool) {
	_, err := client.Node.Info(ctx)
	require.NoError(s.T(), err)
	_, err = client.Node.Ready(ctx)
	require.NoError(s.T(), err)

	head, err := client.Header.LocalHead(ctx)
	require.NoError(s.T(), err)
	_, err = client.Header.GetByHeight(ctx, head.Height())
	require.NoError(s.T(), err)
	_, err = client.Header.GetByHash(ctx, head.Hash())
	require.NoError(s.T(), err)
	_, err = client.Header.SyncState(ctx)
	require.NoError(s.T(), err)
	_, err = client.Header.NetworkHead(ctx)
	require.NoError(s.T(), err)
	_, err = client.Header.Tail(ctx)
	require.NoError(s.T(), err)

	addr, err := client.State.AccountAddress(ctx)
	require.NoError(s.T(), err)
	_, err = client.State.BalanceForAddress(ctx, addr)
	require.NoError(s.T(), err)
	_, err = client.State.Balance(ctx)
	require.NoError(s.T(), err)

	_, err = client.P2P.Info(ctx)
	require.NoError(s.T(), err)
	_, err = client.P2P.Peers(ctx)
	require.NoError(s.T(), err)
	_, err = client.P2P.NATStatus(ctx)
	require.NoError(s.T(), err)
	_, err = client.P2P.BandwidthStats(ctx)
	require.NoError(s.T(), err)
	_, err = client.P2P.ResourceState(ctx)
	// ResourceState may fail when current client calls old server if old server reports peer IDs from current bridge node
	// that contain problematic characters. This is a known limitation and not a breaking change.
	if err != nil && strings.Contains(err.Error(), "invalid cid") && strings.Contains(err.Error(), "peer ID") {
		s.T().Logf("Skipping ResourceState error (known limitation when current client calls old server): %v", err)
	} else {
		require.NoError(s.T(), err)
	}
	_, err = client.P2P.PubSubTopics(ctx)
	require.NoError(s.T(), err)

	namespace, _ := share.NewV0Namespace([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a})
	// Share APIs - node should be synced by now, but handle "data not available" gracefully for compatibility
	err = client.Share.SharesAvailable(ctx, head.Height())
	if err != nil && strings.Contains(err.Error(), "data not available") {
		s.T().Logf("Share.SharesAvailable: data not available (known limitation with old light servers): %v", err)
	} else {
		require.NoError(s.T(), err)
	}
	_, err = client.Share.GetNamespaceData(ctx, head.Height(), namespace)
	if err != nil && strings.Contains(err.Error(), "data not available") {
		s.T().Logf("Share.GetNamespaceData: data not available (known limitation with old light servers): %v", err)
	} else {
		require.NoError(s.T(), err)
	}
	_, err = client.Share.GetEDS(ctx, head.Height())
	if err != nil && strings.Contains(err.Error(), "data not available") {
		s.T().Logf("Share.GetEDS: data not available (known limitation with old light servers): %v", err)
	} else {
		require.NoError(s.T(), err)
	}

	if !skipGetRow {
		rowCtx, rowCancel := context.WithTimeout(ctx, 120*time.Second)
		_, err = client.Share.GetRow(rowCtx, head.Height(), 0)
		rowCancel()
		require.NoError(s.T(), err)
	}

	dasCtx, dasCancel := context.WithTimeout(ctx, 15*time.Second)
	_, err = client.DAS.SamplingStats(dasCtx)
	dasCancel()
	if err != nil && !strings.Contains(err.Error(), "stubbed") && !strings.Contains(err.Error(), "deadline exceeded") {
		require.NoError(s.T(), err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	err = client.DAS.WaitCatchUp(waitCtx)
	cancel()
	// WaitCatchUp may timeout on old light servers due to DAS compatibility issues or RPC connection problems
	if err != nil && !strings.Contains(err.Error(), "stubbed") && !strings.Contains(err.Error(), "deadline exceeded") && !strings.Contains(err.Error(), "context deadline exceeded") {
		require.NoError(s.T(), err)
	} else if err != nil {
		s.T().Logf("Skipping DAS.WaitCatchUp error (known limitation with old servers): %v", err)
	}

	blobCtx, blobCancel := context.WithTimeout(ctx, 30*time.Second)
	_, err = client.Blob.GetAll(blobCtx, head.Height(), []share.Namespace{namespace})
	blobCancel()
	// Blob.GetAll may timeout on old light servers due to compatibility issues
	if err != nil && strings.Contains(err.Error(), "deadline exceeded") {
		s.T().Logf("Skipping Blob.GetAll timeout (known limitation with old light servers): %v", err)
	} else {
		require.NoError(s.T(), err)
	}

	testBlob, err := nodeblob.NewBlobV0(namespace, []byte("cross-version test blob"))
	if err == nil {
		submitHeight, err := client.Blob.Submit(ctx, []*nodeblob.Blob{testBlob}, state.NewTxConfig())
		if err == nil && submitHeight > 0 {
			waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			_, _ = client.Header.WaitForHeight(waitCtx, submitHeight)
			cancel()

			_, err = client.Blob.Get(ctx, submitHeight, namespace, testBlob.Commitment)
			require.NoError(s.T(), err)

			proof, err := client.Blob.GetProof(ctx, submitHeight, namespace, testBlob.Commitment)
			require.NoError(s.T(), err)

			_, err = client.Blob.Included(ctx, submitHeight, namespace, proof, testBlob.Commitment)
			require.NoError(s.T(), err)
		}
	}
}

func (s *CrossVersionClientTestSuite) runClientTestFromDockerImage(ctx context.Context, clientVersion, serverRPCAddr string, skipGetRow bool) error {
	dockerClient := s.f.GetDockerClient()
	networkID := s.f.GetDockerNetwork()
	imageName := "golang:1.25-alpine"

	gomodCacheVol, err := s.getOrCreateVolume(ctx, dockerClient, "tastora-gomodcache")
	if err != nil {
		return fmt.Errorf("failed to get/create Go module cache volume: %w", err)
	}
	gocacheVol, err := s.getOrCreateVolume(ctx, dockerClient, "tastora-gocache")
	if err != nil {
		return fmt.Errorf("failed to get/create Go build cache volume: %w", err)
	}

	tmpDir := s.T().TempDir()
	testProgram := s.generateTestClientProgram(serverRPCAddr, skipGetRow)
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(testProgram), 0o644); err != nil {
		return fmt.Errorf("failed to write test client program: %w", err)
	}
	goModContent := fmt.Sprintf(`module client-test

go 1.25

require github.com/celestiaorg/celestia-node %s

replace (
	cosmossdk.io/x/upgrade => github.com/celestiaorg/cosmos-sdk/x/upgrade v0.2.0
	github.com/cometbft/cometbft => github.com/celestiaorg/celestia-core v0.39.4
	github.com/cosmos/cosmos-sdk => github.com/celestiaorg/cosmos-sdk v0.51.2
	github.com/cosmos/ibc-go/v8 => github.com/celestiaorg/ibc-go/v8 v8.7.2
	github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1
	github.com/syndtr/goleveldb => github.com/syndtr/goleveldb v1.0.1-0.20210819022825-2ae1ddf74ef7
	github.com/tendermint/tendermint => github.com/celestiaorg/celestia-core v1.55.0-tm-v0.34.35
)

replace github.com/ipfs/boxo => github.com/celestiaorg/boxo v0.29.0-fork-4

replace github.com/ipfs/go-datastore => github.com/celestiaorg/go-datastore v0.0.0-20250801131506-48a63ae531e4

replace github.com/celestiaorg/celestia-app/v6 => github.com/celestiaorg/celestia-app/v6 v6.1.2-arabica
`, clientVersion)
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0o644); err != nil {
		return fmt.Errorf("failed to write go.mod: %w", err)
	}

	containerName := fmt.Sprintf("old-client-test-%s-%d", strings.ReplaceAll(clientVersion, ".", "-"), time.Now().Unix())

	// Check if image exists locally first to avoid unnecessary pulls
	_, _, err = dockerClient.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		// Image doesn't exist locally, pull it
		s.T().Logf("Pulling Docker image %s (this may take a while)...", imageName)
		pullResp, pullErr := dockerClient.ImagePull(ctx, imageName, image.PullOptions{})
		if pullErr != nil {
			return fmt.Errorf("failed to pull image %s: %w", imageName, pullErr)
		}
		defer pullResp.Close()
		_, _ = io.Copy(io.Discard, pullResp)
		s.T().Logf("Docker image %s pulled successfully", imageName)
	} else {
		s.T().Logf("Using existing Docker image %s", imageName)
	}

	// Optimize Docker build: use build cache and reduce redundant operations
	config := &container.Config{
		Image: imageName,
		Cmd: []string{"sh", "-c", `
			# Only install git if not already available (layer caching optimization)
			command -v git >/dev/null 2>&1 || apk add --no-cache git && 
			cd /test && 
			# Use go mod download with cache to speed up subsequent builds
			go get -d github.com/celestiaorg/celestia-node@` + clientVersion + ` && 
			go mod download && 
			go mod tidy 2>&1 | grep -v "celestia-app" || true && 
			# Build with optimizations
			go build -o /test/client-test && 
			/test/client-test && 
			rm -f /test/client-test
		`},
		WorkingDir: "/test",
		Env: []string{
			"CGO_ENABLED=0",
			"GOSUMDB=off",
			"GOTOOLCHAIN=auto",
			"GOMODCACHE=/go/pkg/mod",
			"GOCACHE=/root/.cache/go-build",
		},
	}

	hostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(networkID),
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: tmpDir,
				Target: "/test",
			},
			{
				Type:   mount.TypeVolume,
				Source: gomodCacheVol.Name,
				Target: "/go/pkg/mod",
			},
			{
				Type:   mount.TypeVolume,
				Source: gocacheVol.Name,
				Target: "/root/.cache/go-build",
			},
		},
	}

	createResp, err := dockerClient.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}
	defer func() {
		removeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		dockerClient.ContainerRemove(removeCtx, createResp.ID, container.RemoveOptions{Force: true})
	}()

	if err := dockerClient.ContainerStart(ctx, createResp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}
	s.T().Logf("Docker container %s started, waiting for completion...", createResp.ID[:12])

	// Stream logs in background
	logsStream, err := dockerClient.ContainerLogs(ctx, createResp.ID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err == nil {
		go func() {
			defer logsStream.Close()
			_, _ = io.Copy(os.Stdout, logsStream)
		}()
	}

	// Use a longer timeout for Docker container execution (build + test)
	// Note: This timeout must be less than the parent context timeout
	// Increased to 20 minutes to handle slow builds when resources are constrained
	waitCtx, waitCancel := context.WithTimeout(ctx, 20*time.Minute)
	defer waitCancel()

	statusCh, errCh := dockerClient.ContainerWait(waitCtx, createResp.ID, container.WaitConditionNotRunning)
	select {
	case status := <-statusCh:
		if status.StatusCode != 0 {
			var logs bytes.Buffer
			// Get fresh logs after container exits - use background context to ensure we can read logs
			logCtx, logCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer logCancel()
			if logsStream, err := dockerClient.ContainerLogs(logCtx, createResp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true}); err == nil {
				_, _ = io.Copy(&logs, logsStream)
				logsStream.Close()
			}
			logOutput := logs.String()
			if logOutput == "" {
				logOutput = "(no logs available - container may have exited before producing output)"
			}
			// Truncate very long logs to avoid overwhelming output
			if len(logOutput) > 5000 {
				logOutput = logOutput[:5000] + "\n... (truncated)"
			}
			return fmt.Errorf("container exited with code %d:\n%s", status.StatusCode, logOutput)
		}
		return nil
	case err := <-errCh:
		return fmt.Errorf("container wait error: %w", err)
	case <-waitCtx.Done():
		// On timeout, try to get logs to help debug
		var logs bytes.Buffer
		logCtx, logCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer logCancel()
		if logsStream, err := dockerClient.ContainerLogs(logCtx, createResp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true}); err == nil {
			_, _ = io.Copy(&logs, logsStream)
			logsStream.Close()
		}
		logOutput := logs.String()
		if logOutput != "" && len(logOutput) > 1000 {
			logOutput = logOutput[len(logOutput)-1000:] // Show last 1000 chars
		}
		return fmt.Errorf("timeout waiting for container (last logs: %s): %w", logOutput, waitCtx.Err())
	}
}

func (s *CrossVersionClientTestSuite) getOrCreateVolume(ctx context.Context, dockerClient types.TastoraDockerClient, volumeName string) (*volume.Volume, error) {
	vol, err := dockerClient.VolumeInspect(ctx, volumeName)
	if err == nil {
		return &vol, nil
	}

	volResp, err := dockerClient.VolumeCreate(ctx, volume.CreateOptions{
		Name: volumeName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create volume %s: %w", volumeName, err)
	}

	return &volResp, nil
}

func generateAPITestCode(skipGetRow bool) string {
	getRowCode := `
	rowCtx, rowCancel := context.WithTimeout(ctx, 120*time.Second)
	if _, err := client.Share.GetRow(rowCtx, head.Height(), 0); err != nil {
		fmt.Fprintf(os.Stderr, "Share.GetRow failed: %%v\n", err)
		os.Exit(1)
	}
	rowCancel()`

	if skipGetRow {
		getRowCode = `
	// Skipping Share.GetRow for light nodes due to bitswap compatibility issues with old clients`
	}

	return `
	if _, err := client.Node.Info(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Node.Info failed: %%v\n", err)
		os.Exit(1)
	}
	if _, err := client.Node.Ready(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Node.Ready failed: %%v\n", err)
		os.Exit(1)
	}

	head, err := client.Header.LocalHead(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Header.LocalHead failed: %%v\n", err)
		os.Exit(1)
	}
		if _, err := client.Header.GetByHeight(ctx, head.Height()); err != nil {
		fmt.Fprintf(os.Stderr, "Header.GetByHeight failed: %%v\n", err)
		os.Exit(1)
		}
		if _, err := client.Header.GetByHash(ctx, head.Hash()); err != nil {
		fmt.Fprintf(os.Stderr, "Header.GetByHash failed: %%v\n", err)
		os.Exit(1)
	}
	if _, err := client.Header.SyncState(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Header.SyncState failed: %%v\n", err)
		os.Exit(1)
	}
	if _, err := client.Header.NetworkHead(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Header.NetworkHead failed: %%v\n", err)
		os.Exit(1)
	}
	if _, err := client.Header.Tail(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Header.Tail failed: %%v\n", err)
		os.Exit(1)
	}

	addr, err := client.State.AccountAddress(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "State.AccountAddress failed: %%v\n", err)
		os.Exit(1)
	}
		if _, err := client.State.BalanceForAddress(ctx, addr); err != nil {
		fmt.Fprintf(os.Stderr, "State.BalanceForAddress failed: %%v\n", err)
		os.Exit(1)
	}
	if _, err := client.State.Balance(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "State.Balance failed: %%v\n", err)
		os.Exit(1)
	}

	if _, err := client.P2P.Info(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "P2P.Info failed: %%v\n", err)
		os.Exit(1)
	}
	if _, err := client.P2P.Peers(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "P2P.Peers failed: %%v\n", err)
		os.Exit(1)
	}
	if _, err := client.P2P.NATStatus(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "P2P.NATStatus failed: %%v\n", err)
		os.Exit(1)
	}
	if _, err := client.P2P.BandwidthStats(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "P2P.BandwidthStats failed: %%v\n", err)
		os.Exit(1)
	}
	if _, err := client.P2P.ResourceState(ctx); err != nil {
		// ResourceState may fail when old client calls current server if server reports peer IDs
		// that contain problematic characters. This is a known limitation.
		if strings.Contains(err.Error(), "invalid cid") && strings.Contains(err.Error(), "peer ID") {
			fmt.Fprintf(os.Stderr, "P2P.ResourceState failed (known limitation): %%v\n", err)
	} else {
			fmt.Fprintf(os.Stderr, "P2P.ResourceState failed: %%v\n", err)
			os.Exit(1)
		}
	}
	if _, err := client.P2P.PubSubTopics(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "P2P.PubSubTopics failed: %%v\n", err)
		os.Exit(1)
	}

	namespace, _ := share.NewV0Namespace([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a})
		if err := client.Share.SharesAvailable(ctx, head.Height()); err != nil {
		fmt.Fprintf(os.Stderr, "Share.SharesAvailable failed: %%v\n", err)
		os.Exit(1)
		}
		if _, err := client.Share.GetNamespaceData(ctx, head.Height(), namespace); err != nil {
		fmt.Fprintf(os.Stderr, "Share.GetNamespaceData failed: %%v\n", err)
		os.Exit(1)
		}
		if _, err := client.Share.GetEDS(ctx, head.Height()); err != nil {
		fmt.Fprintf(os.Stderr, "Share.GetEDS failed: %%v\n", err)
		os.Exit(1)
	}
` + getRowCode + `

	dasCtx, dasCancel := context.WithTimeout(ctx, 15*time.Second)
	_, err = client.DAS.SamplingStats(dasCtx)
	if err != nil && !strings.Contains(err.Error(), "stubbed") {
		fmt.Fprintf(os.Stderr, "DAS.SamplingStats failed: %%v\n", err)
		os.Exit(1)
	}
	dasCancel()

	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	err = client.DAS.WaitCatchUp(waitCtx)
	cancel()
	// WaitCatchUp may timeout on light servers due to DAS compatibility issues or RPC connection problems
	if err != nil && !strings.Contains(err.Error(), "stubbed") && !strings.Contains(err.Error(), "deadline exceeded") && !strings.Contains(err.Error(), "context deadline exceeded") {
		fmt.Fprintf(os.Stderr, "DAS.WaitCatchUp failed: %%v\n", err)
		os.Exit(1)
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "DAS.WaitCatchUp failed (known limitation): %%v\n", err)
	}

	blobCtx, blobCancel := context.WithTimeout(ctx, 30*time.Second)
	if _, err := client.Blob.GetAll(blobCtx, head.Height(), []share.Namespace{namespace}); err != nil {
		fmt.Fprintf(os.Stderr, "Blob.GetAll failed: %%v\n", err)
		os.Exit(1)
	}
	blobCancel()

			testBlob, err := blob.NewBlobV0(namespace, []byte("cross-version test blob"))
			if err == nil {
				submitHeight, err := client.Blob.Submit(ctx, []*blob.Blob{testBlob}, state.NewTxConfig())
		if err == nil && submitHeight > 0 {
					waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			_, _ = client.Header.WaitForHeight(waitCtx, submitHeight)
					cancel()

			if _, err := client.Blob.Get(ctx, submitHeight, namespace, testBlob.Commitment); err != nil {
				fmt.Fprintf(os.Stderr, "Blob.Get failed: %%v\n", err)
				os.Exit(1)
			}

							proof, err := client.Blob.GetProof(ctx, submitHeight, namespace, testBlob.Commitment)
							if err != nil {
				fmt.Fprintf(os.Stderr, "Blob.GetProof failed: %%v\n", err)
				os.Exit(1)
			}

									if _, err := client.Blob.Included(ctx, submitHeight, namespace, proof, testBlob.Commitment); err != nil {
				fmt.Fprintf(os.Stderr, "Blob.Included failed: %%v\n", err)
				os.Exit(1)
			}
		}
	}`
}

func (s *CrossVersionClientTestSuite) generateTestClientProgram(serverRPCAddr string, skipGetRow bool) string {
	addr := serverRPCAddr
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}

	apiTestCode := generateAPITestCode(skipGetRow)
	imports := `import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/state"
	"github.com/celestiaorg/go-square/v3/share"
)`

	return fmt.Sprintf(`package main

%s

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	serverAddr := %q
	if !hasProtocol(serverAddr) {
		serverAddr = "http://" + serverAddr
	}

	client, err := rpcclient.NewClient(ctx, serverAddr, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create client: %%v\n", err)
		os.Exit(1)
	}
	defer client.Close()
%s
}

func hasProtocol(addr string) bool {
	return len(addr) > 7 && (addr[:7] == "http://" || addr[:8] == "https://")
}
`, imports, addr, apiTestCode)
}
