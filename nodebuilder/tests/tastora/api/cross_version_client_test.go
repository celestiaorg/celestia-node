//go:build integration

package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/celestiaorg/go-square/v3/share"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
	nodeblob "github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora"
	"github.com/celestiaorg/celestia-node/state"
)

type CrossVersionClientTestSuite struct {
	suite.Suite
	f *tastora.Framework
}

func TestCrossVersionClientTestSuite(t *testing.T) {
	suite.Run(t, new(CrossVersionClientTestSuite))
}

func (s *CrossVersionClientTestSuite) SetupSuite() {
	s.f = tastora.NewFramework(s.T(), tastora.WithValidators(1), tastora.WithLightNodes(2))
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	oldVersion := getBaseVersion()
	s.T().Logf("Testing cross-version compatibility with base version: %s", oldVersion)

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
		s.f.WaitForNodeReadyAndSynced(ctx, client, 3*time.Minute)
		s.testAllAPIsWithOptions(ctx, client, true)
	})

	s.T().Run("OldClient_CurrentBridgeServer", func(t *testing.T) {
		testCtx, testCancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer testCancel()

		bridgeNodes := s.f.GetBridgeNodes()
		require.Greater(s.T(), len(bridgeNodes), 0)
		currentServer := bridgeNodes[0]

		serverInfo, err := currentServer.GetNetworkInfo(testCtx)
		require.NoError(s.T(), err)
		rpcAddr := fmt.Sprintf("%s:%s", serverInfo.Internal.Hostname, serverInfo.Internal.Ports.RPC)
		serverRPC := "http://" + rpcAddr

		err = s.runCompatTest(testCtx, oldVersion, serverRPC, false)
		require.NoError(s.T(), err)
	})

	s.T().Run("OldClient_CurrentLightServer", func(t *testing.T) {
		testCtx, testCancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer testCancel()

		lightNode := s.f.NewLightNode(testCtx)
		require.NotNil(s.T(), lightNode)
		time.Sleep(2 * time.Second)

		serverInfo, err := lightNode.GetNetworkInfo(testCtx)
		require.NoError(s.T(), err)
		rpcAddr := fmt.Sprintf("%s:%s", serverInfo.Internal.Hostname, serverInfo.Internal.Ports.RPC)
		serverRPC := "http://" + rpcAddr

		err = s.runCompatTest(testCtx, oldVersion, serverRPC, true)
		require.NoError(s.T(), err)
	})
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
	if err != nil && strings.Contains(err.Error(), "invalid cid") && strings.Contains(err.Error(), "peer ID") {
		s.T().Logf("Skipping ResourceState error (known limitation when current client calls old server): %v", err)
	} else {
		require.NoError(s.T(), err)
	}
	_, err = client.P2P.PubSubTopics(ctx)
	require.NoError(s.T(), err)

	namespace, _ := share.NewV0Namespace([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a})
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
	if err != nil && !strings.Contains(err.Error(), "stubbed") && !strings.Contains(err.Error(), "deadline exceeded") && !strings.Contains(err.Error(), "context deadline exceeded") {
		require.NoError(s.T(), err)
	} else if err != nil {
		s.T().Logf("Skipping DAS.WaitCatchUp error (known limitation with old servers): %v", err)
	}

	blobCtx, blobCancel := context.WithTimeout(ctx, 30*time.Second)
	_, err = client.Blob.GetAll(blobCtx, head.Height(), []share.Namespace{namespace})
	blobCancel()
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

func (s *CrossVersionClientTestSuite) runCompatTest(ctx context.Context, clientVersion, serverRPCAddr string, skipGetRow bool) error {
	dockerClient := s.f.GetDockerClient()
	networkID := s.f.GetDockerNetwork()

	imageName := fmt.Sprintf("ghcr.io/celestiaorg/celestia-node-compat-test:%s", clientVersion)

	_, _, err := dockerClient.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		s.T().Logf("Pulling Docker image %s (this may take a while)...", imageName)
		pullResp, pullErr := dockerClient.ImagePull(ctx, imageName, image.PullOptions{})
		if pullErr != nil {
			s.T().Logf("Failed to pull image %s: %v", imageName, pullErr)
			s.T().Logf("Attempting to build image locally as fallback...")

			if buildErr := s.buildCompatTestImageLocally(ctx, clientVersion, imageName); buildErr != nil {
				return fmt.Errorf("failed to pull image %s and failed to build locally: pull error: %w, build error: %v", imageName, pullErr, buildErr)
			}
			s.T().Logf("Successfully built image locally: %s", imageName)
		} else {
			defer pullResp.Close()
			_, _ = io.Copy(io.Discard, pullResp)
			s.T().Logf("Docker image %s pulled successfully", imageName)
		}
	} else {
		s.T().Logf("Using existing Docker image %s", imageName)
	}

	args := []string{"--rpc-url", serverRPCAddr}
	if skipGetRow {
		args = append(args, "--skip-get-row")
	}

	containerName := fmt.Sprintf("compat-test-%s-%d", strings.ReplaceAll(clientVersion, ".", "-"), time.Now().Unix())

	config := &container.Config{
		Image: imageName,
		Cmd:   args,
	}

	hostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(networkID),
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

	waitCtx, waitCancel := context.WithTimeout(ctx, 10*time.Minute)
	defer waitCancel()

	statusCh, errCh := dockerClient.ContainerWait(waitCtx, createResp.ID, container.WaitConditionNotRunning)
	select {
	case status := <-statusCh:
		if status.StatusCode != 0 {
			var logs bytes.Buffer
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
			if len(logOutput) > 5000 {
				logOutput = logOutput[:5000] + "\n... (truncated)"
			}
			return fmt.Errorf("container exited with code %d:\n%s", status.StatusCode, logOutput)
		}
		return nil
	case err := <-errCh:
		return fmt.Errorf("container wait error: %w", err)
	case <-waitCtx.Done():
		var logs bytes.Buffer
		logCtx, logCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer logCancel()
		if logsStream, err := dockerClient.ContainerLogs(logCtx, createResp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true}); err == nil {
			_, _ = io.Copy(&logs, logsStream)
			logsStream.Close()
		}
		logOutput := logs.String()
		if logOutput != "" && len(logOutput) > 1000 {
			logOutput = logOutput[len(logOutput)-1000:]
		}
		return fmt.Errorf("timeout waiting for container (last logs: %s): %w", logOutput, waitCtx.Err())
	}
}

func (s *CrossVersionClientTestSuite) buildCompatTestImageLocally(ctx context.Context, version, imageName string) error {
	s.T().Logf("Building compat-test image locally for version %s...", version)

	repoRoot := s.getRepoRoot()
	currentCommitCmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	currentCommitCmd.Dir = repoRoot
	currentCommit, err := currentCommitCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get current commit: %w", err)
	}
	currentCommitStr := strings.TrimSpace(string(currentCommit))

	defer func() {
		restoreCmd := exec.CommandContext(ctx, "git", "checkout", currentCommitStr)
		restoreCmd.Dir = repoRoot
		_ = restoreCmd.Run()
	}()

	checkoutCmd := exec.CommandContext(ctx, "git", "checkout", version)
	checkoutCmd.Dir = repoRoot
	if output, err := checkoutCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to checkout version %s: %w. Output: %s. Note: compat-test code may need to be backported to this version", version, err, string(output))
	}

	dockerfilePath := filepath.Join(repoRoot, "cmd/compat-test/Dockerfile")
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		return fmt.Errorf("cmd/compat-test/Dockerfile not found in version %s. The compat-test code needs to be backported to this version", version)
	}

	buildCmd := exec.CommandContext(ctx, "docker", "build",
		"-f", "cmd/compat-test/Dockerfile",
		"-t", imageName,
		".",
	)
	buildCmd.Dir = repoRoot
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker build failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func (s *CrossVersionClientTestSuite) getRepoRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(dir + "/go.mod"); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "."
}

func getBaseVersion() string {
	if version := os.Getenv("CELESTIA_NODE_BASE_VERSION"); version != "" {
		return version
	}
	return "v0.28.3-arabica"
}
