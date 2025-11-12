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
	mobyclient "github.com/moby/moby/client"
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
	s.f = tastora.NewFramework(s.T(), tastora.WithValidators(1), tastora.WithLightNodes(1))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	oldVersion := "v0.28.2-arabica"

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
		s.testAllAPIs(ctx, client)
	})

	s.T().Run("OldClient_CurrentBridgeServer", func(t *testing.T) {
		bridgeNodes := s.f.GetBridgeNodes()
		require.Greater(s.T(), len(bridgeNodes), 0)
		currentServer := bridgeNodes[0]

		serverInfo, err := currentServer.GetNetworkInfo(ctx)
		require.NoError(s.T(), err)
		rpcAddr := fmt.Sprintf("%s:%s", serverInfo.Internal.Hostname, serverInfo.Internal.Ports.RPC)
		serverRPC := "http://" + rpcAddr

		err = s.runClientTestFromDockerImage(ctx, oldVersion, serverRPC, false)
		require.NoError(s.T(), err)
	})

	s.T().Run("OldClient_CurrentLightServer", func(t *testing.T) {
		lightNode := s.f.NewLightNode(ctx)
		require.NotNil(s.T(), lightNode)

		serverInfo, err := lightNode.GetNetworkInfo(ctx)
		require.NoError(s.T(), err)
		rpcAddr := fmt.Sprintf("%s:%s", serverInfo.Internal.Hostname, serverInfo.Internal.Ports.RPC)
		serverRPC := "http://" + rpcAddr

		err = s.runClientTestFromDockerImage(ctx, oldVersion, serverRPC, true)
		require.NoError(s.T(), err)
	})
}

func (s *CrossVersionClientTestSuite) testAllAPIs(ctx context.Context, client *rpcclient.Client) {
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
	require.NoError(s.T(), err)
	_, err = client.P2P.PubSubTopics(ctx)
	require.NoError(s.T(), err)

	namespace, _ := share.NewV0Namespace([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a})
	err = client.Share.SharesAvailable(ctx, head.Height())
	require.NoError(s.T(), err)
	_, err = client.Share.GetNamespaceData(ctx, head.Height(), namespace)
	require.NoError(s.T(), err)
	_, err = client.Share.GetEDS(ctx, head.Height())
	require.NoError(s.T(), err)

	rowCtx, rowCancel := context.WithTimeout(ctx, 120*time.Second)
	_, err = client.Share.GetRow(rowCtx, head.Height(), 0)
	rowCancel()
	require.NoError(s.T(), err)

	dasCtx, dasCancel := context.WithTimeout(ctx, 15*time.Second)
	_, err = client.DAS.SamplingStats(dasCtx)
	if err != nil && !strings.Contains(err.Error(), "stubbed") {
		require.NoError(s.T(), err)
	}
	dasCancel()

	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	err = client.DAS.WaitCatchUp(waitCtx)
	if err != nil && !strings.Contains(err.Error(), "stubbed") {
		require.NoError(s.T(), err)
	}
	cancel()

	blobCtx, blobCancel := context.WithTimeout(ctx, 30*time.Second)
	_, err = client.Blob.GetAll(blobCtx, head.Height(), []share.Namespace{namespace})
	blobCancel()
	require.NoError(s.T(), err)

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
	imageName := "golang:1.24.6-alpine"

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
			/test/client-test && 
			rm -f /test/client-test
		`},
		WorkingDir: "/test",
		Env: []string{
			"CGO_ENABLED=0",
			"GOSUMDB=off",
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

	if logsStream, err := dockerClient.ContainerLogs(ctx, createResp.ID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	}); err == nil {
		defer logsStream.Close()
		_, _ = io.Copy(os.Stdout, logsStream)
	}

	waitCtx, waitCancel := context.WithTimeout(ctx, 5*time.Minute)
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
		return fmt.Errorf("container wait error: %w", err)
	case <-waitCtx.Done():
		return fmt.Errorf("timeout waiting for container: %w", waitCtx.Err())
	}
}

func (s *CrossVersionClientTestSuite) getOrCreateVolume(ctx context.Context, dockerClient *mobyclient.Client, volumeName string) (*volume.Volume, error) {
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
		fmt.Fprintf(os.Stderr, "P2P.ResourceState failed: %%v\n", err)
		os.Exit(1)
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
	if err != nil && !strings.Contains(err.Error(), "stubbed") {
		fmt.Fprintf(os.Stderr, "DAS.WaitCatchUp failed: %%v\n", err)
		os.Exit(1)
	}
	cancel()

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
