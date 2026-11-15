package shwap_test

import (
	"bytes"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

// namespaceFixture extends containerFixture with honest encodings of the
// streamed containers: NamespaceData and RangeNamespaceData.
type namespaceFixture struct {
	*containerFixture
	ods []libshare.Share
	// nsFrom and nsTo bound the fixture namespace in the ODS, [nsFrom, nsTo).
	nsFrom, nsTo int
	nd           []byte
	ranges       []rangeSeed
}

type rangeSeed struct {
	from, to int
	raw      []byte
}

var (
	nsFixtureOnce sync.Once
	nsFixture     *namespaceFixture
)

func getNamespaceFixture(tb testing.TB) *namespaceFixture {
	nsFixtureOnce.Do(func() { nsFixture = newNamespaceFixture(tb) })
	return nsFixture
}

func newNamespaceFixture(tb testing.TB) *namespaceFixture {
	tb.Helper()
	fx := getFixture(tb)
	ctx := tb.Context()

	ods, err := fx.accessor.Shares(ctx)
	require.NoError(tb, err)
	nsFrom, nsTo := -1, -1
	for i, sh := range ods {
		if sh.Namespace().Equals(fx.namespace) {
			if nsFrom == -1 {
				nsFrom = i
			}
			nsTo = i + 1
		}
	}
	require.NotEqual(tb, -1, nsFrom, "the namespace must appear in the ODS")

	nd := honestNamespaceData(tb, fx, fx.namespace)
	var ndBuf bytes.Buffer
	_, err = nd.WriteTo(&ndBuf)
	require.NoError(tb, err)

	nfx := &namespaceFixture{
		containerFixture: fx,
		ods:              ods,
		nsFrom:           nsFrom,
		nsTo:             nsTo,
		nd:               ndBuf.Bytes(),
	}

	// Ranges inside the namespace: the whole namespace, a single share, one
	// that starts and ends mid-row, and one that crosses a row boundary.
	odsSize := fx.odsSize
	for _, r := range [][2]int{
		{nsFrom, nsTo},
		{nsFrom, nsFrom + 1},
		{nsFrom + 1, nsTo - 1},
		{nsFrom + odsSize - 1, nsFrom + odsSize + 1},
	} {
		if r[0] >= r[1] || r[1] > nsTo {
			continue
		}
		nfx.ranges = append(nfx.ranges, rangeSeed{from: r[0], to: r[1], raw: honestRange(tb, fx, r[0], r[1])})
	}
	require.NotEmpty(tb, nfx.ranges)
	return nfx
}

// honestNamespaceData builds the NamespaceData an honest server sends for ns.
func honestNamespaceData(tb testing.TB, fx *containerFixture, ns libshare.Namespace) shwap.NamespaceData {
	rows, err := share.RowsWithNamespace(fx.roots, ns)
	require.NoError(tb, err)
	nd := make(shwap.NamespaceData, 0, len(rows))
	for _, row := range rows {
		rnd, err := fx.accessor.RowNamespaceData(tb.Context(), ns, row)
		require.NoError(tb, err)
		nd = append(nd, rnd)
	}
	return nd
}

// honestRange encodes the response an honest shrex server streams for the
// range [from, to).
func honestRange(tb testing.TB, fx *containerFixture, from, to int) []byte {
	edsID, err := shwap.NewEdsID(1)
	require.NoError(tb, err)
	id, err := shwap.NewRangeNamespaceDataID(edsID, from, to, fx.odsSize)
	require.NoError(tb, err)
	r, err := id.ResponseReader(tb.Context(), &fx.accessor)
	require.NoError(tb, err)
	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(tb, err)
	return buf.Bytes()
}

func sharesEqual(a, b []libshare.Share) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !bytes.Equal(a[i].ToBytes(), b[i].ToBytes()) {
			return false
		}
	}
	return true
}

// FuzzNamespaceDataReadFrom decodes a NamespaceData stream the way the shrex
// getter does and verifies it against the fixture roots. The namespace is
// either the fixture namespace or the one the first decoded share claims, so
// the fuzzer can also aim at namespaces the square does not hold. Data that
// verifies must equal what an honest server sends for that namespace.
func FuzzNamespaceDataReadFrom(f *testing.F) {
	fx := getNamespaceFixture(f)
	f.Add(fx.nd, false)
	f.Add(fx.nd, true)
	f.Add(fx.rnd, false)
	f.Add(append(append([]byte{}, fx.nd...), fx.rnd...), false)
	f.Add(fx.nd[:len(fx.nd)/2], false)
	f.Add([]byte{}, false)
	f.Add([]byte{0}, false)

	f.Fuzz(func(t *testing.T, raw []byte, useClaimedNamespace bool) {
		fx := getNamespaceFixture(t)
		var nd shwap.NamespaceData
		if _, err := nd.ReadFrom(bytes.NewReader(raw)); err != nil {
			return
		}
		_ = nd.Length()
		_ = nd.IsEmpty()

		ns := fx.namespace
		if useClaimedNamespace {
			flat := nd.Flatten()
			if len(flat) == 0 {
				return
			}
			ns = flat[0].Namespace()
			if ns.ValidateForData() != nil {
				return
			}
		}
		if err := nd.Verify(fx.roots, ns); err != nil {
			return
		}
		want := honestNamespaceData(t, fx.containerFixture, ns)
		require.True(t, sharesEqual(want.Flatten(), nd.Flatten()),
			"namespace data that verifies for %s must equal the honest data", ns.String())
	})
}

// FuzzRangeNamespaceDataReadFrom decodes a RangeNamespaceData stream the way
// the shrex getter does and verifies its inclusion for a fuzz-chosen range,
// using the same coordinates and row roots as GetRangeNamespaceData. Data that
// verifies must be exactly the ODS shares of that range.
func FuzzRangeNamespaceDataReadFrom(f *testing.F) {
	fx := getNamespaceFixture(f)
	for _, r := range fx.ranges {
		f.Add(r.raw, uint16(r.from), uint16(r.to-r.from-1))
		// The same response checked against a shifted range.
		f.Add(r.raw, uint16(r.from+1), uint16(r.to-r.from-1))
		f.Add(r.raw, uint16(r.from), uint16(r.to-r.from))
	}
	f.Add(fx.nd, uint16(fx.nsFrom), uint16(fx.nsTo-fx.nsFrom-1))
	f.Add([]byte{}, uint16(0), uint16(0))

	f.Fuzz(func(t *testing.T, raw []byte, fromIdx, length uint16) {
		fx := getNamespaceFixture(t)
		var rngdata shwap.RangeNamespaceData
		if _, err := rngdata.ReadFrom(bytes.NewReader(raw)); err != nil {
			return
		}
		_ = rngdata.IsEmpty()

		odsSize := fx.odsSize
		total := odsSize * odsSize
		from := int(fromIdx) % total
		to := from + 1 + int(length)%(total-from)

		fromCoords, err := shwap.SampleCoordsFrom1DIndex(from, odsSize)
		require.NoError(t, err)
		toCoords, err := shwap.SampleCoordsFrom1DIndex(to-1, odsSize)
		require.NoError(t, err)
		roots := fx.roots.RowRoots[fromCoords.Row : toCoords.Row+1]

		nsErr := rngdata.VerifyNamespace(fromCoords, toCoords, odsSize, roots)
		if err := rngdata.VerifyInclusion(fromCoords, toCoords, odsSize, roots); err != nil {
			require.Error(t, nsErr, "VerifyNamespace accepted data VerifyInclusion rejected")
			return
		}
		require.True(t, sharesEqual(fx.ods[from:to], rngdata.Flatten()),
			"range data that verifies for [%d, %d) must be the ODS shares of that range", from, to)
	})
}

// FuzzRowUnmarshalJSON decodes the JSON form of a Row that an RPC client
// receives from a remote node, then runs the verification a client would. A
// Row that verifies must reproduce the real row.
func FuzzRowUnmarshalJSON(f *testing.F) {
	fx := getFixture(f)
	var row shwap.Row
	_, err := row.ReadFrom(bytes.NewReader(fx.row))
	require.NoError(f, err)
	rowJSON, err := json.Marshal(row)
	require.NoError(f, err)
	shares, err := row.Shares()
	require.NoError(f, err)
	bothJSON, err := json.Marshal(shwap.NewRow(shares, shwap.Both))
	require.NoError(f, err)

	f.Add(rowJSON, uint8(3))
	f.Add(rowJSON, uint8(4))
	f.Add(bothJSON, uint8(3))
	f.Add([]byte(`{"shares":[],"side":"BOTH"}`), uint8(0))
	f.Add([]byte(`{"shares":null,"side":"UP"}`), uint8(0))
	f.Add([]byte(`null`), uint8(0))

	f.Fuzz(func(t *testing.T, raw []byte, rowIdx uint8) {
		fx := getFixture(t)
		idx := int(rowIdx) % (fx.odsSize * 2)
		var row shwap.Row
		if err := json.Unmarshal(raw, &row); err != nil {
			return
		}
		_ = row.IsEmpty()
		if err := row.Verify(fx.roots, idx); err != nil {
			return
		}
		shares, err := row.Shares()
		require.NoError(t, err)
		require.Equal(t, fx.eds.Row(uint(idx)), libshare.ToBytes(shares),
			"a JSON row that verifies at index %d must reproduce the real row", idx)

		out, err := json.Marshal(row)
		require.NoError(t, err)
		var again shwap.Row
		require.NoError(t, json.Unmarshal(out, &again))
		require.NoError(t, again.Verify(fx.roots, idx), "a re-encoded row must still verify")
	})
}

// TestNamespaceFuzzSeeds keeps the seeds meaningful: each honest encoding
// decodes and verifies.
func TestNamespaceFuzzSeeds(t *testing.T) {
	fx := getNamespaceFixture(t)

	var nd shwap.NamespaceData
	_, err := nd.ReadFrom(bytes.NewReader(fx.nd))
	require.NoError(t, err)
	require.NoError(t, nd.Verify(fx.roots, fx.namespace))

	for _, r := range fx.ranges {
		var rngdata shwap.RangeNamespaceData
		_, err := rngdata.ReadFrom(bytes.NewReader(r.raw))
		require.NoError(t, err)
		from, err := shwap.SampleCoordsFrom1DIndex(r.from, fx.odsSize)
		require.NoError(t, err)
		to, err := shwap.SampleCoordsFrom1DIndex(r.to-1, fx.odsSize)
		require.NoError(t, err)
		require.NoError(t, rngdata.VerifyInclusion(from, to, fx.odsSize, fx.roots.RowRoots[from.Row:to.Row+1]),
			"range [%d, %d)", r.from, r.to)
	}
}
