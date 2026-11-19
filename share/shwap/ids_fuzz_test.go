package shwap_test

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/share/shwap"
)

// shwapID is the subset of methods every shwap ID exposes to the wire.
type shwapID interface {
	io.ReaderFrom
	io.WriterTo
	MarshalBinary() ([]byte, error)
	Validate() error
}

// idCodec describes how one ID type is decoded from bytes a remote peer sends.
type idCodec struct {
	name string
	size int
	// newID returns an empty ID to decode into.
	newID func() shwapID
	// fromBinary decodes through the XxxIDFromBinary constructor.
	fromBinary func([]byte) (shwapID, error)
	// validated reports whether a successful decode guarantees Validate passes.
	validated bool
	// verify checks the ID against a square size, if the type supports it. It
	// returns the largest index the ID points at, so the caller can check it
	// is in bounds when verification passes.
	verify func(id shwapID, size int) (maxIdx, bound int, err error)
}

var idCodecs = []idCodec{
	{
		name:  "EdsID",
		size:  shwap.EdsIDSize,
		newID: func() shwapID { return &shwap.EdsID{} },
		fromBinary: func(b []byte) (shwapID, error) {
			id, err := shwap.EdsIDFromBinary(b)
			return &id, err
		},
		validated: true,
	},
	{
		name:  "RowID",
		size:  shwap.RowIDSize,
		newID: func() shwapID { return &shwap.RowID{} },
		fromBinary: func(b []byte) (shwapID, error) {
			id, err := shwap.RowIDFromBinary(b)
			return &id, err
		},
		validated: true,
		verify: func(id shwapID, size int) (int, int, error) {
			rid := id.(*shwap.RowID)
			return rid.RowIndex, size, rid.Verify(size)
		},
	},
	{
		name:  "SampleID",
		size:  shwap.SampleIDSize,
		newID: func() shwapID { return &shwap.SampleID{} },
		fromBinary: func(b []byte) (shwapID, error) {
			id, err := shwap.SampleIDFromBinary(b)
			return &id, err
		},
		validated: true,
		verify: func(id shwapID, size int) (int, int, error) {
			sid := id.(*shwap.SampleID)
			return max(sid.RowIndex, sid.ShareIndex), size, sid.Verify(size)
		},
	},
	{
		name:  "NamespaceDataID",
		size:  shwap.NamespaceDataIDSize,
		newID: func() shwapID { return &shwap.NamespaceDataID{} },
		fromBinary: func(b []byte) (shwapID, error) {
			id, err := shwap.NamespaceDataIDFromBinary(b)
			return &id, err
		},
		validated: true,
	},
	{
		name:  "RowNamespaceDataID",
		size:  shwap.RowNamespaceDataIDSize,
		newID: func() shwapID { return &shwap.RowNamespaceDataID{} },
		fromBinary: func(b []byte) (shwapID, error) {
			id, err := shwap.RowNamespaceDataIDFromBinary(b)
			return &id, err
		},
		// RowNamespaceDataIDFromBinary does not check that the namespace is
		// valid for data; callers run Verify or Validate themselves.
		validated: false,
		verify: func(id shwapID, size int) (int, int, error) {
			rndid := id.(*shwap.RowNamespaceDataID)
			return rndid.RowIndex, size, rndid.Verify(size)
		},
	},
	{
		name:  "RangeNamespaceDataID",
		size:  shwap.RangeNamespaceDataIDSize,
		newID: func() shwapID { return &shwap.RangeNamespaceDataID{} },
		fromBinary: func(b []byte) (shwapID, error) {
			id, err := shwap.RangeNamespaceDataIDFromBinary(b)
			return &id, err
		},
		validated: true,
		verify: func(id shwapID, size int) (int, int, error) {
			rngid := id.(*shwap.RangeNamespaceDataID)
			odsSize := size / 2
			// To is exclusive, so To-1 is the last share index.
			return rngid.To - 1, odsSize * odsSize, rngid.Verify(odsSize)
		},
	},
	{
		name:  "RangeNamespaceDataIDV0",
		size:  shwap.RangeNamespaceDataIDV0Size,
		newID: func() shwapID { return &shwap.RangeNamespaceDataIDV0{} },
		fromBinary: func(b []byte) (shwapID, error) {
			id, err := shwap.RangeNamespaceDataIDV0FromBinary(b)
			return &id, err
		},
		validated: true,
		verify: func(id shwapID, size int) (int, int, error) {
			rngid := id.(*shwap.RangeNamespaceDataIDV0)
			odsSize := size / 2
			return rngid.To - 1, odsSize * odsSize, rngid.Verify(odsSize)
		},
	},
}

// FuzzIDReadFrom decodes every shwap ID type from bytes the way a shrex
// server reads a request off a stream. A decoded ID must re-encode to exactly
// the bytes it was read from, agree with its FromBinary constructor, and, when
// it verifies against a square size, point inside that square.
func FuzzIDReadFrom(f *testing.F) {
	ns := libshare.MustNewV0Namespace(bytes.Repeat([]byte{0x42}, libshare.NamespaceVersionZeroIDSize))
	edsID, err := shwap.NewEdsID(7)
	require.NoError(f, err)
	rowID, err := shwap.NewRowID(7, 3, 16)
	require.NoError(f, err)
	sampleID, err := shwap.NewSampleID(7, shwap.SampleCoords{Row: 3, Col: 5}, 16)
	require.NoError(f, err)
	ndID, err := shwap.NewNamespaceDataID(7, ns)
	require.NoError(f, err)
	rndID, err := shwap.NewRowNamespaceDataID(7, 3, ns, 16)
	require.NoError(f, err)
	rngID, err := shwap.NewRangeNamespaceDataID(edsID, 2, 30, 8)
	require.NoError(f, err)
	rngIDV0, err := shwap.NewRangeNamespaceDataIDV0(edsID, 2, 30, 8)
	require.NoError(f, err)

	valid := []interface{ MarshalBinary() ([]byte, error) }{edsID, rowID, sampleID, ndID, rndID, rngID, rngIDV0}
	for kind, id := range valid {
		raw, err := id.MarshalBinary()
		require.NoError(f, err)
		require.Len(f, raw, idCodecs[kind].size, idCodecs[kind].name)
		f.Add(uint8(kind), raw, uint8(4))
		// Out-of-bounds coordinates for a small square.
		f.Add(uint8(kind), raw, uint8(1))
		// Trailing bytes after the ID.
		f.Add(uint8(kind), append(raw, 0xff), uint8(4))
		// Truncated.
		f.Add(uint8(kind), raw[:len(raw)-1], uint8(4))
	}
	maxed := bytes.Repeat([]byte{0xff}, shwap.RangeNamespaceDataIDSize+libshare.NamespaceSize)
	zeros := make([]byte, len(maxed))
	heightOnly := binary.BigEndian.AppendUint64(nil, 1)
	for kind := range idCodecs {
		f.Add(uint8(kind), maxed, uint8(7))
		f.Add(uint8(kind), zeros, uint8(0))
		f.Add(uint8(kind), heightOnly, uint8(3))
		f.Add(uint8(kind), []byte{}, uint8(0))
	}

	f.Fuzz(func(t *testing.T, kind uint8, raw []byte, sizeLog uint8) {
		codec := idCodecs[int(kind)%len(idCodecs)]

		id := codec.newID()
		n, err := id.ReadFrom(bytes.NewReader(raw))
		fromBinary, binErr := codec.fromBinary(raw[:min(len(raw), codec.size)])
		if err != nil {
			require.False(t, len(raw) >= codec.size && binErr == nil,
				"%s: ReadFrom failed (%v) but FromBinary accepted the same bytes", codec.name, err)
			return
		}
		require.NoError(t, binErr, "%s: FromBinary rejected bytes ReadFrom accepted", codec.name)
		require.EqualValues(t, codec.size, n, "%s: ReadFrom consumed a wrong amount", codec.name)

		if codec.validated {
			require.NoError(t, id.Validate(), "%s: decoded ID does not validate", codec.name)
		} else {
			_ = id.Validate()
		}

		encoded, err := id.MarshalBinary()
		require.NoError(t, err)
		require.Equal(t, raw[:codec.size], encoded, "%s: decode/encode is not a round trip", codec.name)
		fromBinaryEncoded, err := fromBinary.MarshalBinary()
		require.NoError(t, err)
		require.Equal(t, encoded, fromBinaryEncoded, "%s: ReadFrom and FromBinary disagree", codec.name)

		var buf bytes.Buffer
		if _, err := id.WriteTo(&buf); err == nil {
			require.Equal(t, encoded, buf.Bytes(), "%s: WriteTo and MarshalBinary disagree", codec.name)
		}

		if codec.verify == nil {
			return
		}
		// Square sizes are powers of two; also try zero and non-powers.
		for _, size := range []int{0, 1, 3, 1 << (sizeLog % 10)} {
			maxIdx, bound, err := codec.verify(id, size)
			if err != nil {
				continue
			}
			require.Less(t, maxIdx, bound, "%s: verified for size %d but points outside it", codec.name, size)
		}
	})
}
