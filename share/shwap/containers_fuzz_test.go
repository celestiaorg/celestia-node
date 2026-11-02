package shwap_test

import (
	"bytes"
	"io"
	"math/rand/v2"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v10/pkg/wrapper"
	libshare "github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

// containerFixture is a pseudo-random square with a namespace that spans a few
// rows, its roots, and honest encodings of one container of each kind. It is
// built once per process from a fixed seed, so every fuzz worker and every
// replay of a saved corpus entry sees the same square as the seeds were
// encoded against. The fuzz targets only read from it.
type containerFixture struct {
	odsSize   int
	eds       *rsmt2d.ExtendedDataSquare
	roots     *share.AxisRoots
	accessor  eds.Rsmt2D
	namespace libshare.Namespace
	nsRow     int // a row whose roots span the namespace
	sample    []byte
	row       []byte
	rnd       []byte
}

var (
	fixtureOnce sync.Once
	fixture     *containerFixture
)

func newContainerFixture(tb testing.TB) *containerFixture {
	tb.Helper()
	const odsSize = 8
	namespace := libshare.MustNewV0Namespace(bytes.Repeat([]byte{0x42}, libshare.NamespaceVersionZeroIDSize))
	square, roots := deterministicEDSWithNamespace(tb, namespace, odsSize*3, odsSize)
	accessor := eds.Rsmt2D{ExtendedDataSquare: square}

	encode := func(w io.WriterTo) []byte {
		var buf bytes.Buffer
		_, err := w.WriteTo(&buf)
		require.NoError(tb, err)
		return buf.Bytes()
	}

	sample, err := accessor.SampleForProofAxis(shwap.SampleCoords{Row: 1, Col: 2}, rsmt2d.Row)
	require.NoError(tb, err)
	row, err := accessor.HalfRow(3, shwap.Left)
	require.NoError(tb, err)
	// The namespaced shares sort into some rows of the square; pick the first row that holds them.
	nsRow := -1
	var rnd shwap.RowNamespaceData
	for idx := range odsSize {
		candidate, err := accessor.RowNamespaceData(tb.Context(), namespace, idx)
		if err != nil {
			continue // the row's roots do not span the namespace
		}
		if len(candidate.Shares) > 0 {
			nsRow, rnd = idx, candidate
			break
		}
	}
	require.NotEqual(tb, -1, nsRow, "the namespace must appear in the square")

	return &containerFixture{
		odsSize:   odsSize,
		eds:       square,
		roots:     roots,
		accessor:  accessor,
		namespace: namespace,
		nsRow:     nsRow,
		sample:    encode(&sample),
		row:       encode(&row),
		rnd:       encode(&rnd),
	}
}

// deterministicEDSWithNamespace mirrors edstest.RandEDSWithNamespace but draws
// from a fixed-seed generator instead of crypto/rand.
func deterministicEDSWithNamespace(
	tb testing.TB,
	namespace libshare.Namespace,
	namespacedAmount, odsSize int,
) (*rsmt2d.ExtendedDataSquare, *share.AxisRoots) {
	tb.Helper()
	rng := rand.New(rand.NewChaCha8([32]byte{'s', 'h', 'w', 'a', 'p'}))
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

func getFixture(tb testing.TB) *containerFixture {
	fixtureOnce.Do(func() { fixture = newContainerFixture(tb) })
	return fixture
}

// FuzzSampleReadFrom decodes a Sample the way the shrex client does,
// verifies it at fuzz-chosen coordinates, and checks that a Sample which
// verifies is the share that really sits at those coordinates.
func FuzzSampleReadFrom(f *testing.F) {
	fx := getFixture(f)
	f.Add(fx.sample, uint8(1), uint8(2))
	f.Add(fx.sample, uint8(2), uint8(1))
	f.Add([]byte{}, uint8(0), uint8(0))
	f.Fuzz(func(t *testing.T, raw []byte, rowIdx, colIdx uint8) {
		fx := getFixture(t)
		var s shwap.Sample
		if _, err := s.ReadFrom(bytes.NewReader(raw)); err != nil {
			return
		}
		size := fx.odsSize * 2
		r, c := int(rowIdx)%size, int(colIdx)%size
		if err := s.Verify(fx.roots, r, c); err != nil {
			return
		}
		require.Equal(t, fx.eds.GetCell(uint(r), uint(c)), s.ToBytes(),
			"a sample that verifies at (%d,%d) must be the share at those coordinates", r, c)
	})
}

// FuzzRowReadFrom decodes a Row, verifies it against the roots at a fuzz-chosen
// row index, and checks that a Row which verifies reproduces the real row.
func FuzzRowReadFrom(f *testing.F) {
	fx := getFixture(f)
	f.Add(fx.row, uint8(3))
	f.Add(fx.row, uint8(4))
	f.Add([]byte{}, uint8(0))
	f.Fuzz(func(t *testing.T, raw []byte, rowIdx uint8) {
		fx := getFixture(t)
		var row shwap.Row
		if _, err := row.ReadFrom(bytes.NewReader(raw)); err != nil {
			return
		}
		idx := int(rowIdx) % (fx.odsSize * 2)
		if err := row.Verify(fx.roots, idx); err != nil {
			return
		}
		shares, err := row.Shares()
		require.NoError(t, err)
		require.Equal(t, fx.eds.Row(uint(idx)), libshare.ToBytes(shares),
			"a row that verifies at index %d must reproduce the real row", idx)
	})
}

// FuzzRowNamespaceDataReadFrom decodes RowNamespaceData, verifies it for the
// fixture namespace at a fuzz-chosen row, and checks that verified data equals
// the honest namespace data for that row.
func FuzzRowNamespaceDataReadFrom(f *testing.F) {
	fx := getFixture(f)
	f.Add(fx.rnd, uint8(fx.nsRow))
	f.Add(fx.rnd, uint8(fx.nsRow+1))
	f.Add([]byte{}, uint8(0))
	f.Fuzz(func(t *testing.T, raw []byte, rowIdx uint8) {
		fx := getFixture(t)
		var rnd shwap.RowNamespaceData
		if _, err := rnd.ReadFrom(bytes.NewReader(raw)); err != nil {
			return
		}
		idx := int(rowIdx) % (fx.odsSize * 2)
		if err := rnd.Verify(fx.roots, fx.namespace, idx); err != nil {
			return
		}
		want, err := fx.accessor.RowNamespaceData(t.Context(), fx.namespace, idx)
		require.NoError(t, err)
		require.True(t, slices.EqualFunc(want.Shares, rnd.Shares, func(a, b libshare.Share) bool {
			return bytes.Equal(a.ToBytes(), b.ToBytes())
		}), "namespace data that verifies at row %d must equal the honest data", idx)
	})
}

// TestContainerFuzzSeeds keeps the seeds meaningful: each honest encoding
// decodes and verifies at its own coordinates.
func TestContainerFuzzSeeds(t *testing.T) {
	fx := getFixture(t)

	var s shwap.Sample
	_, err := s.ReadFrom(bytes.NewReader(fx.sample))
	require.NoError(t, err)
	require.NoError(t, s.Verify(fx.roots, 1, 2))
	require.Error(t, s.Verify(fx.roots, 2, 1))

	var row shwap.Row
	_, err = row.ReadFrom(bytes.NewReader(fx.row))
	require.NoError(t, err)
	require.NoError(t, row.Verify(fx.roots, 3))
	require.Error(t, row.Verify(fx.roots, 4))

	var rnd shwap.RowNamespaceData
	_, err = rnd.ReadFrom(bytes.NewReader(fx.rnd))
	require.NoError(t, err)
	require.NoError(t, rnd.Verify(fx.roots, fx.namespace, fx.nsRow))
	require.Error(t, rnd.Verify(fx.roots, fx.namespace, (fx.nsRow+1)%(fx.odsSize*2)))

	// Fuzz workers and corpus replays rebuild the fixture in their own process.
	rebuilt := newContainerFixture(t)
	require.Equal(t, fx.roots.Hash(), rebuilt.roots.Hash(), "the fixture square must be deterministic")
	require.Equal(t, fx.sample, rebuilt.sample)
	require.Equal(t, fx.row, rebuilt.row)
	require.Equal(t, fx.rnd, rebuilt.rnd)
}
