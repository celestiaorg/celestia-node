package e2e

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"testing"
	"time"

	"github.com/celestiaorg/knuu/pkg/k8s"
	"github.com/celestiaorg/knuu/pkg/knuu"
	"github.com/celestiaorg/knuu/pkg/minio"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestBlobModule(t *testing.T) {
	var (
		ctx    = context.Background()
		logger = logrus.New()
	)

	k8sClient, err := k8s.NewClient(ctx, knuu.DefaultTestScope(), logger)
	require.NoError(t, err)

	minioClient, err := minio.New(ctx, k8sClient)
	require.NoError(t, err)

	kn, err := knuu.New(ctx, knuu.Options{
		K8sClient:    k8sClient,
		MinioClient:  minioClient,
		ProxyEnabled: true,
	})
	require.NoError(t, err)

	kn.HandleStopSignal(ctx)

	t.Cleanup(func() {
		if err := kn.CleanUp(ctx); err != nil {
			t.Logf("error cleaning up knuu: %v", err)
		}
	})

	executor, err := createExecutor(ctx, kn)
	require.NoError(t, err)

	app, err := createAndStartApp(ctx, kn)
	require.NoError(t, err)

	require.NoError(t, waitForHeight(ctx, executor, app, 1))

	bridgeNode, err := createBridge(ctx, kn, "bridge", executor, app)
	require.NoError(t, err)

	require.NoError(t, bridgeNode.Start(ctx))

	lightNode, err := createNode(ctx, kn, "light", nodeTypeLight, executor, app, bridgeNode)
	require.NoError(t, err)

	require.NoError(t, lightNode.Start(ctx))

	rpcClientLight, err := getDaNodeRPCClient(ctx, bridgeNode)
	require.NoError(t, err)

	infoLight, err := rpcClientLight.Node.Info(ctx)
	fmt.Printf("\nerr: %v\n", err)
	t.Logf("Light info: %s, %t, %s", infoLight.Type.String(), infoLight.Type.IsValid(), infoLight.APIVersion)

	// ----------------------
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	select {
	case <-time.After(60 * time.Minute):
		t.Log("Waited for 60 minutes.")
	case <-sigChan:
		t.Log("Received interrupt signal, stopping wait.")
	}

	return
	// ----------------------

	require.NoError(t, err)

	t.Logf("Light info: %s, %t, %s", infoLight.Type.String(), infoLight.Type.IsValid(), infoLight.APIVersion)
}
