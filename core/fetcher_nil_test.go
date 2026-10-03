package core

import (
	"context"
	"testing"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	coregrpc "github.com/cometbft/cometbft/rpc/grpc"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type missingSyncInfoClient struct {
	coregrpc.BlockAPIClient
}

func (c *missingSyncInfoClient) Status(
	context.Context,
	*coregrpc.StatusRequest,
	...grpc.CallOption,
) (*coregrpc.StatusResponse, error) {
	return &coregrpc.StatusResponse{}, nil
}

func TestBlockFetcherRejectsMissingSyncInfo(t *testing.T) {
	fetcher := &BlockFetcher{client: &missingSyncInfoClient{}}

	_, err := fetcher.IsSyncing(context.Background())
	require.EqualError(t, err, "core/fetcher: sync info not available in status response")
}

func TestPartsToBlockRejectsNilPart(t *testing.T) {
	_, err := partsToBlock([]*tmproto.Part{nil})
	require.EqualError(t, err, "core/fetcher: block part at position 0 is nil")
}
