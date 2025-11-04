//go:build integration

package api

import (
	"bytes"
	"context"
	_ "embed"
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
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora"
)

//go:embed cross_version_client_test_template.go
var testClientTemplate string

type CrossVersionClientTestSuite struct {
	suite.Suite
	f *tastora.Framework
}

func TestCrossVersionClientTestSuite(t *testing.T) {
	suite.Run(t, new(CrossVersionClientTestSuite))
}

func (s *CrossVersionClientTestSuite) SetupSuite() {
	s.f = tastora.NewFramework(s.T(), tastora.WithValidators(1), tastora.WithLightNodes(1))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	err := s.f.SetupNetwork(ctx)
	require.NoError(s.T(), err, "Failed to setup network")
	time.Sleep(5 * time.Second)
}

func (s *CrossVersionClientTestSuite) TearDownSuite() {
	if s.f != nil {
		s.f.Stop(context.Background())
	}
}

func (s *CrossVersionClientTestSuite) TestCrossVersionBidirectional() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	oldVersion := "v0.28.2-arabica"

	s.T().Run("CurrentClient_OldBridgeServer", func(t *testing.T) {
		oldServer := s.f.NewBridgeNodeWithVersion(ctx, oldVersion)
		require.NotNil(s.T(), oldServer)
		time.Sleep(2 * time.Second)

		client := s.f.GetNodeRPCClient(ctx, oldServer)
		results := s.testAllAPIs(ctx, client, "CurrentClient_OldBridgeServer")
		s.T().Logf("Current client → old bridge server %s: %d passed, %d failed", oldVersion, results.passed, results.failed)
		if results.failed > 0 {
			s.T().Fail()
		}
	})

	s.T().Run("CurrentClient_OldLightServer", func(t *testing.T) {
		oldServer := s.f.NewLightNodeWithVersion(ctx, oldVersion)
		require.NotNil(s.T(), oldServer)
		time.Sleep(2 * time.Second)

		client := s.f.GetNodeRPCClient(ctx, oldServer)
		results := s.testAllAPIs(ctx, client, "CurrentClient_OldLightServer")
		s.T().Logf("Current client → old light server %s: %d passed, %d failed", oldVersion, results.passed, results.failed)
		if results.failed > 0 {
			s.T().Fail()
		}
	})

	s.T().Run("OldClient_CurrentBridgeServer", func(t *testing.T) {
		bridgeNodes := s.f.GetBridgeNodes()
		require.Greater(s.T(), len(bridgeNodes), 0)
		currentServer := bridgeNodes[0]
		time.Sleep(3 * time.Second)

		serverInfo, err := currentServer.GetNetworkInfo(ctx)
		require.NoError(s.T(), err)
		rpcAddr := fmt.Sprintf("%s:%s", serverInfo.Internal.Hostname, serverInfo.Internal.Ports.RPC)
		serverRPC := "http://" + rpcAddr

		err = s.runClientTestFromDockerImage(ctx, oldVersion, serverRPC)
		require.NoError(s.T(), err)
		s.T().Logf("✓ Old client %s → current bridge server: PASS", oldVersion)
	})

	s.T().Run("OldClient_CurrentLightServer", func(t *testing.T) {
		lightNode := s.f.NewLightNode(ctx)
		require.NotNil(s.T(), lightNode)
		time.Sleep(3 * time.Second)

		serverInfo, err := lightNode.GetNetworkInfo(ctx)
		require.NoError(s.T(), err)
		rpcAddr := fmt.Sprintf("%s:%s", serverInfo.Internal.Hostname, serverInfo.Internal.Ports.RPC)
		serverRPC := "http://" + rpcAddr

		err = s.runClientTestFromDockerImage(ctx, oldVersion, serverRPC)
		require.NoError(s.T(), err)
		s.T().Logf("✓ Old client %s → current light server: PASS", oldVersion)
	})
}

func (s *CrossVersionClientTestSuite) runClientTestFromDockerImage(ctx context.Context, clientVersion, serverRPCAddr string) error {
	dockerClient := s.f.GetDockerClient()
	networkID := s.f.GetDockerNetwork()
	imageName := "golang:1.24.6-alpine"

	tmpDir := s.T().TempDir()
	testProgram := s.generateTestClientProgram(serverRPCAddr)
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(testProgram), 0o644); err != nil {
		return fmt.Errorf("failed to write test client program: %w", err)
	}
	goModContent := fmt.Sprintf(`module client-test

go 1.24.6

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

	s.T().Logf("Pulling Docker image: %s", imageName)
	pullResp, err := dockerClient.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", imageName, err)
	}
	defer pullResp.Close()
	_, _ = io.Copy(io.Discard, pullResp)

	config := &container.Config{
		Image: imageName,
		Cmd: []string{"sh", "-c", `
			apk add --no-cache git && cd /test && 
			go get -d github.com/celestiaorg/celestia-node@` + clientVersion + ` && 
			go mod download && 
			go mod tidy 2>&1 | grep -v "celestia-app" || true && 
			go build -o /test/client-test && 
			/test/client-test
		`},
		WorkingDir: "/test",
		Env:        []string{"CGO_ENABLED=0", "GOSUMDB=off"},
	}

	hostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(networkID),
		Mounts: []mount.Mount{{
			Type:   mount.TypeBind,
			Source: tmpDir,
			Target: "/test",
		}},
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

	if logsStream, err := dockerClient.ContainerLogs(ctx, createResp.ID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	}); err == nil {
		defer logsStream.Close()
		_, _ = io.Copy(os.Stdout, logsStream)
	}

	waitCtx, waitCancel := context.WithTimeout(ctx, 10*time.Minute)
	defer waitCancel()

	statusCh, errCh := dockerClient.ContainerWait(waitCtx, createResp.ID, container.WaitConditionNotRunning)
	select {
	case status := <-statusCh:
		if status.StatusCode != 0 {
			var logs bytes.Buffer
			if logsStream, err := dockerClient.ContainerLogs(ctx, createResp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true}); err == nil {
				_, _ = io.Copy(&logs, logsStream)
				logsStream.Close()
			}
			return fmt.Errorf("container exited with code %d:\n%s", status.StatusCode, logs.String())
		}
		return nil
	case err := <-errCh:
		var logs bytes.Buffer
		if logsStream, logErr := dockerClient.ContainerLogs(ctx, createResp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true}); logErr == nil {
			_, _ = io.Copy(&logs, logsStream)
			logsStream.Close()
		}
		return fmt.Errorf("container wait error: %w\nContainer logs:\n%s", err, logs.String())
	case <-waitCtx.Done():
		var logs bytes.Buffer
		if logsStream, err := dockerClient.ContainerLogs(ctx, createResp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true}); err == nil {
			_, _ = io.Copy(&logs, logsStream)
			logsStream.Close()
		}
		return fmt.Errorf("timeout waiting for container after 10m: %w\nContainer logs:\n%s", waitCtx.Err(), logs.String())
	}
}

type apiTestResults struct {
	passed int
	failed int
}

func (s *CrossVersionClientTestSuite) testAllAPIs(ctx context.Context, client *client.Client, testName string) apiTestResults {
	var results apiTestResults

	// Node API
	if _, err := client.Node.Info(ctx); err != nil {
		s.T().Logf("✗ Node.Info failed: %v", err)
		results.failed++
	} else {
		s.T().Logf("✓ Node.Info")
		results.passed++
	}
	if _, err := client.Node.Ready(ctx); err != nil {
		s.T().Logf("✗ Node.Ready failed: %v", err)
		results.failed++
	} else {
		s.T().Logf("✓ Node.Ready")
		results.passed++
	}

	// Header API
	head, err := client.Header.LocalHead(ctx)
	if err != nil {
		s.T().Logf("✗ Header.LocalHead failed: %v", err)
		results.failed++
	} else {
		s.T().Logf("✓ Header.LocalHead: Height=%d", head.Height())
		results.passed++
		if _, err := client.Header.GetByHeight(ctx, head.Height()); err != nil {
			s.T().Logf("✗ Header.GetByHeight failed: %v", err)
			results.failed++
		} else {
			s.T().Logf("✓ Header.GetByHeight")
			results.passed++
		}
		if _, err := client.Header.GetByHash(ctx, head.Hash()); err != nil {
			s.T().Logf("✗ Header.GetByHash failed: %v", err)
			results.failed++
		} else {
			s.T().Logf("✓ Header.GetByHash")
			results.passed++
		}
	}
	if _, err := client.Header.SyncState(ctx); err != nil {
		s.T().Logf("✗ Header.SyncState failed: %v", err)
		results.failed++
	} else {
		s.T().Logf("✓ Header.SyncState")
		results.passed++
	}
	if _, err := client.Header.NetworkHead(ctx); err != nil {
		s.T().Logf("✗ Header.NetworkHead failed: %v", err)
		results.failed++
	} else {
		s.T().Logf("✓ Header.NetworkHead")
		results.passed++
	}
	if _, err := client.Header.Tail(ctx); err != nil {
		s.T().Logf("✗ Header.Tail failed: %v", err)
		results.failed++
	} else {
		s.T().Logf("✓ Header.Tail")
		results.passed++
	}

	// State API
	addr, err := client.State.AccountAddress(ctx)
	if err != nil {
		s.T().Logf("✗ State.AccountAddress failed: %v", err)
		results.failed++
	} else {
		s.T().Logf("✓ State.AccountAddress")
		results.passed++
		if _, err := client.State.BalanceForAddress(ctx, addr); err != nil {
			s.T().Logf("✗ State.BalanceForAddress failed: %v", err)
			results.failed++
		} else {
			s.T().Logf("✓ State.BalanceForAddress")
			results.passed++
		}
	}
	if _, err := client.State.Balance(ctx); err != nil {
		s.T().Logf("✗ State.Balance failed: %v", err)
		results.failed++
	} else {
		s.T().Logf("✓ State.Balance")
		results.passed++
	}

	// P2P API
	if _, err := client.P2P.Info(ctx); err != nil {
		s.T().Logf("✗ P2P.Info failed: %v", err)
		results.failed++
	} else {
		s.T().Logf("✓ P2P.Info")
		results.passed++
	}
	if _, err := client.P2P.Peers(ctx); err != nil {
		s.T().Logf("✗ P2P.Peers failed: %v", err)
		results.failed++
	} else {
		s.T().Logf("✓ P2P.Peers")
		results.passed++
	}
	if _, err := client.P2P.NATStatus(ctx); err != nil {
		s.T().Logf("✗ P2P.NATStatus failed: %v", err)
		results.failed++
	} else {
		s.T().Logf("✓ P2P.NATStatus")
		results.passed++
	}
	if _, err := client.P2P.BandwidthStats(ctx); err != nil {
		s.T().Logf("✗ P2P.BandwidthStats failed: %v", err)
		results.failed++
	} else {
		s.T().Logf("✓ P2P.BandwidthStats")
		results.passed++
	}
	if _, err := client.P2P.ResourceState(ctx); err != nil {
		s.T().Logf("✗ P2P.ResourceState failed: %v", err)
		results.failed++
	} else {
		s.T().Logf("✓ P2P.ResourceState")
		results.passed++
	}
	if _, err := client.P2P.PubSubTopics(ctx); err != nil {
		s.T().Logf("✗ P2P.PubSubTopics failed: %v", err)
		results.failed++
	} else {
		s.T().Logf("✓ P2P.PubSubTopics")
		results.passed++
	}

	// Share API
	namespace, _ := share.NewV0Namespace([]byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0})
	if head != nil {
		if err := client.Share.SharesAvailable(ctx, head.Height()); err != nil {
			s.T().Logf("✗ Share.SharesAvailable failed: %v", err)
			results.failed++
		} else {
			s.T().Logf("✓ Share.SharesAvailable")
			results.passed++
		}
		if _, err := client.Share.GetNamespaceData(ctx, head.Height(), namespace); err != nil {
			s.T().Logf("✗ Share.GetNamespaceData failed: %v", err)
			results.failed++
		} else {
			s.T().Logf("✓ Share.GetNamespaceData")
			results.passed++
		}
	}

	// DAS API
	if _, err := client.DAS.SamplingStats(ctx); err != nil {
		s.T().Logf("✗ DAS.SamplingStats failed: %v", err)
		results.failed++
	} else {
		s.T().Logf("✓ DAS.SamplingStats")
		results.passed++
	}
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.DAS.WaitCatchUp(waitCtx); err != nil && err != context.DeadlineExceeded {
		s.T().Logf("✗ DAS.WaitCatchUp failed: %v", err)
		results.failed++
	} else {
		s.T().Logf("✓ DAS.WaitCatchUp")
		results.passed++
	}

	// Blob API
	if head != nil {
		if _, err := client.Blob.GetAll(ctx, head.Height(), []share.Namespace{namespace}); err != nil {
			s.T().Logf("✗ Blob.GetAll failed: %v", err)
			results.failed++
		} else {
			s.T().Logf("✓ Blob.GetAll")
			results.passed++
		}
		// Blob.Included requires proof and commitment, skip for now
	}

	return results
}

func (s *CrossVersionClientTestSuite) generateTestClientProgram(serverRPCAddr string) string {
	addr := serverRPCAddr
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}
	program := strings.ReplaceAll(testClientTemplate, `//go:build ignore`, ``)
	program = strings.ReplaceAll(program, `SERVER_ADDR_PLACEHOLDER`, addr)
	return program
}
