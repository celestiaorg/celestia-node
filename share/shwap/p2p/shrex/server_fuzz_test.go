package shrex

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v10/pkg/wrapper"
	libshare "github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/store"
)

const (
	fuzzODSSize = 8
	// fuzzHeightQ4 holds the square as an ODS+Q4 file, the layout of recent
	// blocks; fuzzHeightODS holds it as an ODS-only file, the archival layout.
	fuzzHeightQ4  = 1
	fuzzHeightODS = 2
)

// serverFixture is a store holding one deterministic square at two heights,
// served through the same store accessors a shrex server uses.
type serverFixture struct {
	store     *store.Store
	square    *rsmt2d.ExtendedDataSquare
	roots     *share.AxisRoots
	ods       []libshare.Share
	namespace libshare.Namespace
}

func newServerFixture(f *testing.F) *serverFixture {
	f.Helper()
	ctx := context.Background()
	namespace := libshare.MustNewV0Namespace(bytes.Repeat([]byte{0x42}, libshare.NamespaceVersionZeroIDSize))
	square, roots := deterministicSquare(f, namespace, fuzzODSSize*3, fuzzODSSize)

	s, err := store.NewStore(store.DefaultParameters(), f.TempDir())
	require.NoError(f, err)
	f.Cleanup(func() { _ = s.Stop(ctx) })
	require.NoError(f, s.PutODSQ4(ctx, roots, fuzzHeightQ4, square))
	require.NoError(f, s.PutODS(ctx, roots, fuzzHeightODS, square))

	ods, err := libshare.FromBytes(square.FlattenedODS())
	require.NoError(f, err)
	return &serverFixture{store: s, square: square, roots: roots, ods: ods, namespace: namespace}
}

// deterministicSquare builds a square from a fixed-seed generator, so every
// fuzz worker and every replay of a saved input serves the same data.
func deterministicSquare(
	tb testing.TB,
	namespace libshare.Namespace,
	namespacedAmount, odsSize int,
) (*rsmt2d.ExtendedDataSquare, *share.AxisRoots) {
	tb.Helper()
	rng := rand.New(rand.NewChaCha8([32]byte{'s', 'h', 'r', 'e', 'x'}))
	shares := make([]libshare.Share, odsSize*odsSize)
	for i := range shares {
		raw := make([]byte, libshare.ShareSize)
		ns := namespace
		if i >= namespacedAmount {
			id := make([]byte, libshare.NamespaceVersionZeroIDSize)
			for j := range id {
				id[j] = byte(rng.Uint32())
			}
			var err error
			ns, err = libshare.NewV0Namespace(id)
			require.NoError(tb, err)
		}
		copy(raw[:libshare.NamespaceSize], ns.Bytes())
		for j := libshare.NamespaceSize; j < len(raw); j++ {
			raw[j] = byte(rng.Uint32())
		}
		sh, err := libshare.NewShare(raw)
		require.NoError(tb, err)
		shares[i] = sh
	}
	slices.SortFunc(shares, func(a, b libshare.Share) int { return bytes.Compare(a.ToBytes(), b.ToBytes()) })

	square, err := rsmt2d.ComputeExtendedDataSquare(
		libshare.ToBytes(shares),
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(odsSize)),
	)
	require.NoError(tb, err)
	roots, err := share.NewAxisRoots(square)
	require.NoError(tb, err)
	return square, roots
}

// registryIndex returns the position of the request type with the given name.
func registryIndex(tb testing.TB, name string) uint8 {
	for i, newReq := range registry {
		if newReq().Name() == name {
			return uint8(i)
		}
	}
	tb.Fatalf("request %q is not registered", name)
	return 0
}

// FuzzServerRequest feeds request bytes to every registered shrex request type
// and runs the server's handling of them against a real store: decode,
// Validate, memory reservation sizing, and streaming the response. Whenever
// the server produces a response, the client-side decoding and verification of
// it must succeed, since the server is honest here.
func FuzzServerRequest(f *testing.F) {
	fx := newServerFixture(f)

	ns := fx.namespace
	edsSize := fuzzODSSize * 2
	seeds := make([]request, 0, 10)
	for _, height := range []uint64{fuzzHeightQ4, fuzzHeightODS} {
		eid, err := shwap.NewEdsID(height)
		require.NoError(f, err)
		ndID, err := shwap.NewNamespaceDataID(height, ns)
		require.NoError(f, err)
		sampleID, err := shwap.NewSampleID(height, shwap.SampleCoords{Row: 3, Col: 11}, edsSize)
		require.NoError(f, err)
		rowID, err := shwap.NewRowID(height, 12, edsSize)
		require.NoError(f, err)
		rngID, err := shwap.NewRangeNamespaceDataID(eid, 2, 20, fuzzODSSize)
		require.NoError(f, err)
		seeds = append(seeds, &eid, &ndID, &sampleID, &rowID, &rngID)
	}
	for _, req := range seeds {
		var buf bytes.Buffer
		_, err := req.WriteTo(&buf)
		require.NoError(f, err)
		kind := registryIndex(f, req.Name())
		f.Add(kind, buf.Bytes())
		// The same bytes parsed as every other request type.
		for other := range registry {
			if uint8(other) != kind {
				f.Add(uint8(other), buf.Bytes())
			}
		}
	}
	f.Add(uint8(0), []byte{})

	f.Fuzz(func(t *testing.T, kind uint8, raw []byte) {
		ctx := t.Context()
		req := registry[int(kind)%len(registry)]()

		// Server: Server.handleDataRequest.
		if _, err := req.ReadFrom(bytes.NewReader(raw)); err != nil {
			return
		}
		if err := req.Validate(); err != nil {
			return
		}
		file, err := fx.store.GetByHeight(ctx, req.Height())
		if errors.Is(err, store.ErrNotFound) {
			return
		}
		require.NoError(t, err)
		defer file.Close()
		size, err := file.Size(ctx)
		require.NoError(t, err)
		require.Equal(t, edsSize, size)
		require.Positive(t, req.ResponseSize(size), "memory reservation for %s must be positive", req.Name())

		r, err := req.ResponseReader(ctx, file)
		if err != nil {
			return
		}
		var resp bytes.Buffer
		if _, err := io.Copy(&resp, r); err != nil {
			return
		}

		// Client: the shrex getter's decoding and verification of the response.
		switch id := req.(type) {
		case *shwap.EdsID:
			_, err := eds.ReadAccessor(ctx, &resp, fx.roots)
			require.NoError(t, err, "honest EDS response must verify")
		case *shwap.NamespaceDataID:
			var nd shwap.NamespaceData
			_, err := nd.ReadFrom(&resp)
			require.NoError(t, err)
			require.NoError(t, nd.Verify(fx.roots, id.DataNamespace), "honest namespace data must verify")
		case *shwap.SampleID:
			var sample shwap.Sample
			_, err := sample.ReadFrom(&resp)
			require.NoError(t, err)
			require.NoError(t, sample.Verify(fx.roots, id.RowIndex, id.ShareIndex), "honest sample must verify")
		case *shwap.RowID:
			var row shwap.Row
			_, err := row.ReadFrom(&resp)
			require.NoError(t, err)
			require.NoError(t, row.Verify(fx.roots, id.RowIndex), "honest row must verify")
		case *shwap.RangeNamespaceDataID:
			var rngdata shwap.RangeNamespaceData
			_, err := rngdata.ReadFrom(&resp)
			require.NoError(t, err)
			from, err := shwap.SampleCoordsFrom1DIndex(id.From, fuzzODSSize)
			require.NoError(t, err)
			to, err := shwap.SampleCoordsFrom1DIndex(id.To-1, fuzzODSSize)
			require.NoError(t, err)
			require.NoError(t,
				rngdata.VerifyInclusion(from, to, fuzzODSSize, fx.roots.RowRoots[from.Row:to.Row+1]),
				"honest range [%d, %d) must verify", id.From, id.To)
			require.Equal(t, libshare.ToBytes(fx.ods[id.From:id.To]), libshare.ToBytes(rngdata.Flatten()))
		default:
			t.Fatalf("no client-side check for request %s", req.Name())
		}
	})
}
