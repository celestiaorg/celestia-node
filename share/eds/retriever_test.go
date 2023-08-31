package eds

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ipfs/go-blockservice"
	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestRetriever_Retrieve(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bServ := mdutils.Bserv()
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
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// generate EDS
			shares := sharetest.RandShares(t, tc.squareSize*tc.squareSize)
			in, err := ipld.AddShares(ctx, shares, bServ)
			require.NoError(t, err)

			// limit with timeout, specifically retrieval
			ctx, cancel := context.WithTimeout(ctx, time.Minute*5) // the timeout is big for the max size which is long
			defer cancel()

			dah, err := da.NewDataAvailabilityHeader(in)
			require.NoError(t, err)
			out, err := r.Retrieve(ctx, &dah)
			require.NoError(t, err)
			assert.True(t, in.Equals(out))
		})
	}
}

func TestRetriever_ByzantineError(t *testing.T) {
	const width = 8
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	bserv := mdutils.Bserv()
	shares := edstest.RandEDS(t, width).Flattened()
	_, err := ipld.ImportShares(ctx, shares, bserv)
	require.NoError(t, err)

	// corrupt shares so that eds erasure coding does not match
	copy(shares[14][share.NamespaceSize:], shares[15][share.NamespaceSize:])

	// import corrupted eds
	batchAdder := ipld.NewNmtNodeAdder(ctx, bserv, ipld.MaxSizeBatchOption(width*2))
	attackerEDS, err := rsmt2d.ImportExtendedDataSquare(
		shares,
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(width),
			nmt.NodeVisitor(batchAdder.Visit)),
	)
	require.NoError(t, err)
	err = batchAdder.Commit()
	require.NoError(t, err)

	// ensure we rcv an error
	dah, err := da.NewDataAvailabilityHeader(attackerEDS)
	require.NoError(t, err)
	r := NewRetriever(bserv)
	_, err = r.Retrieve(ctx, &dah)
	var errByz *byzantine.ErrByzantine
	require.ErrorAs(t, err, &errByz)
}

// TestRetriever_MultipleRandQuadrants asserts that reconstruction succeeds
// when any three random quadrants requested.
func TestRetriever_MultipleRandQuadrants(t *testing.T) {
	RetrieveQuadrantTimeout = time.Millisecond * 500
	const squareSize = 32
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	bServ := mdutils.Bserv()
	r := NewRetriever(bServ)

	// generate EDS
	shares := sharetest.RandShares(t, squareSize*squareSize)
	in, err := ipld.AddShares(ctx, shares, bServ)
	require.NoError(t, err)

	dah, err := da.NewDataAvailabilityHeader(in)
	require.NoError(t, err)
	ses, err := r.newSession(ctx, &dah)
	require.NoError(t, err)

	// wait until two additional quadrants requested
	// this reliably allows us to reproduce the issue
	time.Sleep(RetrieveQuadrantTimeout * 2)
	// then ensure we have enough shares for reconstruction for slow machines e.g. CI
	<-ses.Done()

	_, err = ses.Reconstruct(ctx)
	assert.NoError(t, err)
}

func TestFraudProofValidation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer t.Cleanup(cancel)
	bServ := mdutils.Bserv()

	odsSize := []int{2, 4, 16, 32, 64, 128}
	for _, size := range odsSize {
		t.Run(fmt.Sprintf("ods size:%d", size), func(t *testing.T) {
			var errByz *byzantine.ErrByzantine
			faultHeader, err := generateByzantineError(ctx, t, size, bServ)
			require.True(t, errors.As(err, &errByz))

			p := byzantine.CreateBadEncodingProof([]byte("hash"), faultHeader.Height(), errByz)
			err = p.Validate(faultHeader)
			require.NoError(t, err)
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
BenchmarkBEFPValidation/ods_size:2         	   31273	     38819 ns/op	   68052 B/op	     366 allocs/op
BenchmarkBEFPValidation/ods_size:4         	   14664	     80439 ns/op	  135892 B/op	     894 allocs/op
BenchmarkBEFPValidation/ods_size:16       	    2850	    386178 ns/op	  587890 B/op	    4945 allocs/op
BenchmarkBEFPValidation/ods_size:32        	    1399	    874490 ns/op	 1233399 B/op	   11284 allocs/op
BenchmarkBEFPValidation/ods_size:64        	     619	   2047540 ns/op	 2578008 B/op	   25364 allocs/op
BenchmarkBEFPValidation/ods_size:128       	     259	   4934375 ns/op	 5418406 B/op	   56345 allocs/op
*/
func BenchmarkBEFPValidation(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer b.Cleanup(cancel)
	bServ := mdutils.Bserv()
	r := NewRetriever(bServ)
	t := &testing.T{}
	odsSize := []int{2, 4, 16, 32, 64, 128}
	for _, size := range odsSize {
		b.Run(fmt.Sprintf("ods size:%d", size), func(b *testing.B) {
			b.ResetTimer()
			b.StopTimer()
			eds := edstest.RandByzantineEDS(t, size)
			err := ipld.ImportEDS(ctx, eds, bServ)
			require.NoError(t, err)
			h := headertest.ExtendedHeaderFromEDS(t, 1, eds)
			_, err = r.Retrieve(ctx, h.DAH)
			var errByz *byzantine.ErrByzantine
			require.ErrorAs(t, err, &errByz)
			b.StartTimer()

			for i := 0; i < b.N; i++ {
				b.ReportAllocs()
				p := byzantine.CreateBadEncodingProof([]byte("hash"), h.Height(), errByz)
				err = p.Validate(h)
				require.NoError(b, err)
			}
		})
	}
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
	bServ := mdutils.Bserv()
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

			for i := 0; i < b.N; i++ {
				err = byzantine.NewErrByzantine(ctx, bServ, h.DAH, errByz)
				require.NotNil(t, err)
			}
		})
	}
}
