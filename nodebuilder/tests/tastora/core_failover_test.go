//go:build integration

package tastora

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	rpcclient "github.com/celestiaorg/celestia-node/api/rpc/client"
)

// CoreFailoverTestSuite runs one bridge fed by two core endpoints (validator[0] as the primary
// and validator[1] as an additional endpoint in config.toml). Helps to test that network will not
// be halted if at least one core endpoint is healthy.
type CoreFailoverTestSuite struct {
	suite.Suite
	framework *Framework
}

func TestCoreFailoverTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping core-failover integration tests in short mode")
	}

	suite.Run(t, &CoreFailoverTestSuite{})
}

func (s *CoreFailoverTestSuite) SetupSuite() {
	s.framework = NewFramework(s.T(),
		WithValidators(4), WithBridgeNodes(1), WithLightNodes(0), WithMultiSource())
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
}

func (s *CoreFailoverTestSuite) TearDownSuite() {
	if s.framework != nil {
		s.framework.Cleanup()
	}
}

// TestBridgeCoreFailover verifies that a bridge configured with a primary plus one additional
// core endpoint keeps ingesting new blocks when either endpoint dies in both directions.
func (s *CoreFailoverTestSuite) TestBridgeCoreFailover() {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	bridge := s.framework.GetBridgeNodes()[0]
	client := s.framework.GetNodeRPCClient(ctx, bridge)

	// both endpoints up the bridge ingests normally.
	s.requireHeadAdvances(ctx, client, 3, 90*time.Second, "baseline: bridge should ingest with both core endpoints up")

	// kill the primary. The chain stays live - the head continue growing from the additional endpoint
	s.framework.StopValidator(ctx, 0)
	s.requireHeadAdvances(ctx, client, 3, 4*time.Minute,
		"bridge should keep ingesting from the additional endpoint after the primary dies")

	// restore the primary and once it is caught up and re-joined as a source - kill the
	// additional. Continued head growth now can only come from the primary.
	s.framework.StartValidator(ctx, 0)
	s.framework.WaitValidatorSynced(ctx, 0, 2*time.Minute)
	// Give the bridge's fetcher time to resubscribe to the recovered primary
	s.requireHeadAdvances(ctx, client, 2, 90*time.Second, "bridge should still ingest after the primary recovers")

	s.framework.StopValidator(ctx, 1)
	s.requireHeadAdvances(ctx, client, 3, 4*time.Minute,
		"bridge should ingest from the recovered primary after the additional dies")
}

// requireHeadAdvances asserts the bridge's local head grows by at least `by` heights within timeout.
func (s *CoreFailoverTestSuite) requireHeadAdvances(
	ctx context.Context,
	client *rpcclient.Client,
	by uint64,
	timeout time.Duration,
	msg string,
) {
	head, err := client.Header.LocalHead(ctx)
	s.Require().NoError(err, "should read bridge local head")
	target := head.Height() + by
	s.Require().Eventually(func() bool {
		h, err := client.Header.LocalHead(ctx)
		return err == nil && h.Height() >= target
	}, timeout, 2*time.Second, msg)
}
