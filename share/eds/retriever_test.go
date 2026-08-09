package eds

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ipfs/boxo/blockservice"
	blocks "github.com/ipfs/go-block-format"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/ipld"
)

func TestRetriever_Retrieve(t *testing.T) {
	// TODO @node-team: figure out why this regressed in CI
	t.Skip("skipping retrieval as dangling component")
	baseCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bServ := ipld.NewMemBlockservice()
	r := NewRetriever(bServ)

	type test struct {
		name       string
		squareSize int
	}
	tests := []test{
		{"1x1(min)", 1},
		{"2x2(med)", 2},
		{"4x4(med)", 4},
		{"8x8(med)", 8},
		{"16x16(med)", 16},
		{"32x32(med)", 32},
		{"64x64(med)", 64},
		{"128x128(max)", share.MaxSquareSize},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// generate EDS
			shares, err := libshare.RandShares(tc.squareSize * tc.squareSize)
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(baseCtx, time.Minute*5) // generous timeout for large squares
			t.Cleanup(cancel)
			in, err := ipld.AddShares(ctx, shares, bServ)
			require.NoError(t, err)

			roots, err := share.NewAxisRoots(in)
			require.NoError(t, err)
			out, err := r.Retrieve(ctx, roots)
			require.NoError(t, err)
			assert.True(t, in.Equals(out))
		})
	}
}

// TestRetriever_MultipleRandQuadrants asserts that reconstruction succeeds
// when any three random quadrants requested.
func TestRetriever_MultipleRandQuadrants(t *testing.T) {
	RetrieveQuadrantTimeout = time.Millisecond * 500
	const squareSize = 32
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	bServ := ipld.NewMemBlockservice()
	r := NewRetriever(bServ)

	// generate EDS
	shares, err := libshare.RandShares(squareSize * squareSize)
	require.NoError(t, err)
	in, err := ipld.AddShares(ctx, shares, bServ)
	require.NoError(t, err)

	roots, err := share.NewAxisRoots(in)
	require.NoError(t, err)
	ses, err := r.newSession(ctx, roots)
	require.NoError(t, err)

	// wait until two additional quadrants requested
	// this reliably allows us to reproduce the issue
	time.Sleep(RetrieveQuadrantTimeout * 2)
	// then ensure we have enough shares for reconstruction for slow machines e.g. CI
	<-ses.Done()

	_, err = ses.Reconstruct(ctx)
	assert.NoError(t, err)
}

func TestRetriever_PersistsIntermediateNodesAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{}, 1)
	gate := make(chan struct{})
	bServ := &gatedBlockService{
		BlockService: ipld.NewMemBlockservice(),
		started:      started,
		gate:         gate,
	}
	ses, roots, flattened, width := newPartialRetrievalSession(t, ctx, bServ)

	reconstructed := make(chan error, 1)
	go func() {
		_, err := ses.Reconstruct(ctx)
		reconstructed <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		close(gate)
		t.Fatal("timed out waiting for intermediate NMT write")
	}
	cancel()
	close(gate)
	require.ErrorIs(t, <-reconstructed, rsmt2d.ErrUnrepairableDataSquare)
	ses.close(false)

	gotShare, err := ipld.GetShare(
		context.Background(),
		bServ,
		ipld.MustCidFromNamespacedSha256(roots.RowRoots[3]),
		0,
		width,
	)
	require.NoError(t, err)
	require.Equal(t, flattened[12], gotShare.ToBytes())
}

func TestRetriever_IntermediateNodeCommitIsBounded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{}, 1)
	canceled := make(chan struct{}, 1)
	gate := make(chan struct{})
	defer close(gate)
	bServ := &gatedBlockService{
		BlockService: ipld.NewMemBlockservice(),
		started:      started,
		canceled:     canceled,
		gate:         gate,
	}
	ses, _, _, _ := newPartialRetrievalSession(t, ctx, bServ)
	ses.commitTimeout = 10 * time.Millisecond

	reconstructed := make(chan error, 1)
	go func() {
		_, err := ses.Reconstruct(ctx)
		reconstructed <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for intermediate NMT write")
	}
	cancel()
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("timed out canceling intermediate NMT write")
	}
	select {
	case err := <-reconstructed:
		require.ErrorIs(t, err, rsmt2d.ErrUnrepairableDataSquare)
	case <-time.After(time.Second):
		t.Fatal("timed out reconstructing after cancellation")
	}
	closed := make(chan struct{})
	go func() {
		ses.close(false)
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("timed out closing retrieval session")
	}
}

func newPartialRetrievalSession(
	t *testing.T,
	ctx context.Context,
	bServ blockservice.BlockService,
) (*retrievalSession, *share.AxisRoots, [][]byte, int) {
	t.Helper()
	eds := edstest.RandEDS(t, 2)
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	ses, err := NewRetriever(bServ).newSession(ctx, roots)
	require.NoError(t, err)
	flattened := eds.Flattened()
	width := int(eds.Width())
	// These cells repair row 3 while leaving the rest of the square unrepairable.
	for _, index := range []int{11, 14, 15} {
		row, column := index/width, index%width
		require.NoError(t, ses.square.SetCell(uint(row), uint(column), flattened[index]))
	}
	return ses, roots, flattened, width
}

type gatedBlockService struct {
	blockservice.BlockService
	started  chan<- struct{}
	canceled chan<- struct{}
	gate     <-chan struct{}
}

func (b *gatedBlockService) AddBlocks(ctx context.Context, blks []blocks.Block) error {
	select {
	case b.started <- struct{}{}:
	default:
	}
	select {
	case <-b.gate:
	case <-ctx.Done():
		select {
		case b.canceled <- struct{}{}:
		default:
		}
		return ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return b.BlockService.AddBlocks(ctx, blks)
}

func TestByzantineError(t *testing.T) {
	bServ := ipld.NewMemBlockservice()

	odsSize := []int{2, 4, 16, 32, 64, 128}
	for _, size := range odsSize {
		t.Run(fmt.Sprintf("ods size:%d", size), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
			t.Cleanup(cancel)

			var errByz *byzantine.ErrByzantine
			_, err := generateByzantineError(ctx, t, size, bServ)
			require.NotNil(t, err)
			require.True(t, errors.As(err, &errByz), err.Error())
		})
	}
}

func generateByzantineError(
	ctx context.Context,
	t *testing.T,
	odsSize int,
	bServ blockservice.BlockService,
) (*header.ExtendedHeader, error) {
	eds := edstest.RandByzantineEDS(t, odsSize)
	err := ipld.ImportEDS(ctx, eds, bServ)
	require.NoError(t, err)
	h := headertest.ExtendedHeaderFromEDS(t, 1, eds)
	_, err = NewRetriever(bServ).Retrieve(ctx, h.DAH)

	return h, err
}

/*
BenchmarkNewErrByzantineData/ods_size:2        	   29605	     38846 ns/op	   49518 B/op	     579 allocs/op
BenchmarkNewErrByzantineData/ods_size:4      	   11380	    105302 ns/op	  134967 B/op	    1571 allocs/op
BenchmarkNewErrByzantineData/ods_size:16       	    1902	    631086 ns/op	  830199 B/op	    9601 allocs/op
BenchmarkNewErrByzantineData/ods_size:32        	 756	   1530985 ns/op	 1985272 B/op	   22901 allocs/op
BenchmarkNewErrByzantineData/ods_size:64       	     340	   3445544 ns/op	 4767053 B/op	   54704 allocs/op
BenchmarkNewErrByzantineData/ods_size:128      	     132	   8740678 ns/op	11991093 B/op	  136584 allocs/op
*/
func BenchmarkNewErrByzantineData(b *testing.B) {
	odsSize := []int{2, 4, 16, 32, 64, 128}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	bServ := ipld.NewMemBlockservice()
	r := NewRetriever(bServ)
	t := &testing.T{}
	for _, size := range odsSize {
		b.Run(fmt.Sprintf("ods size:%d", size), func(b *testing.B) {
			b.StopTimer()
			eds := edstest.RandByzantineEDS(t, size)
			err := ipld.ImportEDS(ctx, eds, bServ)
			require.NoError(t, err)
			h := headertest.ExtendedHeaderFromEDS(t, 1, eds)
			ses, err := r.newSession(ctx, h.DAH)
			require.NoError(t, err)

			select {
			case <-ctx.Done():
				b.Fatal(ctx.Err())
			case <-ses.Done():
			}

			_, err = ses.Reconstruct(ctx)
			assert.NoError(t, err)
			var errByz *rsmt2d.ErrByzantineData
			require.ErrorAs(t, err, &errByz)
			b.StartTimer()

			for b.Loop() {
				err = byzantine.NewErrByzantine(ctx, bServ.Blockstore(), h.DAH, errByz)
				require.NotNil(t, err)
			}
		})
	}
}
