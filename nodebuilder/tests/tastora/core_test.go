//go:build integration

package tastora

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// CoreControlTestSuite runs a healthy bridge (which proves the chain is up) alongside an extra bridge
// started against an unreachable core to reach the core-unreachable startup path.
type CoreControlTestSuite struct {
	suite.Suite
	framework *Framework
}

func TestCoreControlTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping core-control integration tests in short mode")
	}
	suite.Run(t, &CoreControlTestSuite{})
}

func (s *CoreControlTestSuite) SetupSuite() {
	s.framework = NewFramework(s.T(), WithValidators(1), WithBridgeNodes(2), WithLightNodes(0))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
}

func (s *CoreControlTestSuite) TearDownSuite() {
	if s.framework != nil {
		s.framework.Cleanup()
	}
}

// TestBridgeCoreUnreachable starts a bridge pointed at an unreachable core and asserts the observable
// startup contract: the node does not hang forever.
func (s *CoreControlTestSuite) TestBridgeCoreUnreachable() {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	node := s.framework.StartBridgeNodeWithBrokenCore(ctx)

	// Bridge StartupTimeout defaults to 2m; poll well past it to catch a hang without flaking on a
	// slow container/store startup.
	s.Require().Eventually(func() bool {
		return !s.framework.bridgeContainerRunning(ctx, node)
	}, 4*time.Minute, 5*time.Second, "bridge with an unreachable core must terminate, not hang indefinitely")

	exitCode, logs := s.framework.bridgeContainerExit(ctx, node)
	s.Assert().NotZero(exitCode, "a failed startup must exit non-zero")
	s.Assert().Contains(strings.ToLower(logs), "failed to start",
		"logs should carry a clear startup failure, got:\n%s", logs)
}
