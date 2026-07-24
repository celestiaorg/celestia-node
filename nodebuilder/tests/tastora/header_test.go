//go:build integration

package tastora

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// HeaderTestSuite provides testing of the Header module APIs against a live network.
type HeaderTestSuite struct {
	suite.Suite
	framework *Framework
}

func TestHeaderTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Header module integration tests in short mode")
	}
	suite.Run(t, &HeaderTestSuite{})
}

func (s *HeaderTestSuite) SetupSuite() {
	s.framework = NewFramework(s.T(), WithValidators(1), WithBridgeNodes(1))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	s.Require().NoError(s.framework.SetupNetwork(ctx))
}

func (s *HeaderTestSuite) TearDownSuite() {
	if s.framework != nil {
		s.framework.Cleanup()
	}
}

// TestHeaderLiveSync subscribes to the bridge's header stream and asserts that
// it produces a live chain.
func (s *HeaderTestSuite) TestHeaderLiveSync() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	client := s.framework.GetNodeRPCClientWS(ctx, bridgeNode)

	sub, err := client.Header.Subscribe(ctx)
	s.Require().NoError(err)

	const want = 5
	var prev uint64
	for i := 0; i < want; i++ {
		select {
		case eh := <-sub:
			s.Require().NotNil(eh)
			s.Assert().Greater(eh.Height(), prev, "heights should strictly increase")
			s.Assert().NotEmpty(eh.DAH.RowRoots, "header at height %d should carry a non-empty DAH", eh.Height())
			prev = eh.Height()
		case <-ctx.Done():
			s.FailNowf("timed out waiting for header events", "received %d of %d", i, want)
		}
	}
}

// TestHeaderSubscribe asserts that the subscription delivers canonical, retrievable
// headers
func (s *HeaderTestSuite) TestHeaderSubscribe() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	bridgeNode := s.framework.GetBridgeNodes()[0]
	client := s.framework.GetNodeRPCClientWS(ctx, bridgeNode)

	sub, err := client.Header.Subscribe(ctx)
	s.Require().NoError(err)

	const want = 5
	for i := 0; i < want; i++ {
		select {
		case eh := <-sub:
			s.Require().NotNil(eh)
			got, err := client.Header.GetByHeight(ctx, eh.Height())
			s.Require().NoError(err)
			s.Assert().Equal(eh.Hash(), got.Hash(), "streamed header at height %d should match GetByHeight", eh.Height())
		case <-ctx.Done():
			s.FailNowf("timed out waiting for header events", "received %d of %d", i, want)
		}
	}
}
