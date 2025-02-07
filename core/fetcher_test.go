package core

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coregrpc "github.com/tendermint/tendermint/rpc/grpc"
	"google.golang.org/grpc"
)

func TestBlockFetcher_GetBlock_and_SubscribeNewBlockEvent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	t.Cleanup(cancel)

	host, port, err := net.SplitHostPort(StartTestNode(t).GRPCClient.Target())
	require.NoError(t, err)
	client := newTestClient(t, host, port)
	fetcher, err := NewBlockFetcher(client)
	require.NoError(t, err)
	// generate some blocks
	newBlockChan, err := fetcher.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)

	for i := 1; i < 3; i++ {
		select {
		case newBlockFromChan := <-newBlockChan:
			h := newBlockFromChan.Header.Height
			block, err := fetcher.GetSignedBlock(ctx, h)
			require.NoError(t, err)
			assert.Equal(t, newBlockFromChan.Data, *block.Data)
			assert.Equal(t, newBlockFromChan.Header, *block.Header)
			assert.Equal(t, newBlockFromChan.Commit, *block.Commit)
			assert.Equal(t, newBlockFromChan.ValidatorSet, *block.ValidatorSet)
			require.GreaterOrEqual(t, newBlockFromChan.Header.Height, int64(i))
		case <-ctx.Done():
			require.NoError(t, ctx.Err())
		}
	}
	require.NoError(t, fetcher.Stop(ctx))
}

type mockAPIService struct {
	coregrpc.UnimplementedBlockAPIServer

	grpcServer *grpc.Server
	fetcher    *BlockFetcher
}

func (m *mockAPIService) SubscribeNewHeights(
	_ *coregrpc.SubscribeNewHeightsRequest,
	srv coregrpc.BlockAPI_SubscribeNewHeightsServer,
) error {
	for i := 0; i < 20; i++ {
		b, err := m.fetcher.GetBlock(context.Background(), int64(i))
		if err != nil {
			return err
		}
		err = srv.Send(&coregrpc.NewHeightEvent{Height: b.Header.Height, Hash: b.Header.Hash()})
		if err != nil {
			return err
		}
		time.Sleep(time.Second)
	}
	return nil
}

func (m *mockAPIService) BlockByHeight(
	req *coregrpc.BlockByHeightRequest,
	srv coregrpc.BlockAPI_BlockByHeightServer,
) error {
	b, err := m.fetcher.client.BlockByHeight(context.Background(), &coregrpc.BlockByHeightRequest{Height: req.Height})
	if err != nil {
		return err
	}
	data, err := b.Recv()
	if err != nil {
		return err
	}
	err = srv.Send(data)
	if err != nil {
		return err
	}
	return nil
}

func (m *mockAPIService) Start() error {
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()
	m.grpcServer = grpcServer
	coregrpc.RegisterBlockAPIServer(grpcServer, m)
	go func() {
		err = grpcServer.Serve(listener)
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			panic(err)
		}
	}()
	return nil
}

func (m *mockAPIService) Stop() error {
	m.grpcServer.Stop()
	return nil
}

func (m *mockAPIService) generateBlocksWithHeights(ctx context.Context, t *testing.T) {
	cfg := DefaultTestConfig()
	fetcher, cctx := createCoreFetcher(t, cfg)
	m.fetcher = fetcher
	generateNonEmptyBlocks(t, ctx, fetcher, cfg, cctx)
	require.NoError(t, fetcher.Stop(ctx))
}

func TestStart_SubscribeNewBlockEvent_Resubscription(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)
	m := &mockAPIService{}
	m.generateBlocksWithHeights(ctx, t)

	require.NoError(t, m.Start())

	client := newTestClient(t, "localhost", "50051")

	fetcher, err := NewBlockFetcher(client)
	require.NoError(t, err)
	newBlockChan, err := fetcher.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)

	select {
	case newBlockFromChan := <-newBlockChan:
		h := newBlockFromChan.Header.Height
		_, err := fetcher.GetSignedBlock(ctx, h)
		require.NoError(t, err)
	case <-ctx.Done():
		require.NoError(t, ctx.Err())
	}

	require.NoError(t, m.Stop())
	_, ok := <-newBlockChan
	require.False(t, ok)
	require.NoError(t, m.Start())
	newBlockChan, err = fetcher.SubscribeNewBlockEvent(ctx)
	require.NoError(t, err)
	select {
	case newBlockFromChan := <-newBlockChan:
		h := newBlockFromChan.Header.Height
		_, err := fetcher.GetSignedBlock(ctx, h)
		require.NoError(t, err)
	case <-ctx.Done():
		require.NoError(t, ctx.Err())
	}
	require.NoError(t, m.Stop())
	require.NoError(t, m.fetcher.Stop(ctx))
}
