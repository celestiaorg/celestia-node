package shwap_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/go-libp2p-messenger/serde"
	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/pb"
)

// TestProtoDecodersRejectMalformedInput guards against a class of remote-DoS
// where a peer sends proto containers with nil/invalid Share fields. Decoders
// must return an error rather than producing a zero-value libshare.Share, which
// would panic later when Namespace()/ToBytes() are invoked on it.
func TestProtoDecodersRejectMalformedInput(t *testing.T) {
	t.Run("ShareFromProto/nil", func(t *testing.T) {
		_, err := shwap.ShareFromProto(nil)
		require.Error(t, err)
	})

	t.Run("ShareFromProto/empty data", func(t *testing.T) {
		_, err := shwap.ShareFromProto(&pb.Share{Data: nil})
		require.Error(t, err)
	})

	t.Run("ShareFromProto/wrong size", func(t *testing.T) {
		_, err := shwap.ShareFromProto(&pb.Share{Data: []byte{1, 2, 3}})
		require.Error(t, err)
	})

	t.Run("SharesFromProto/nil element", func(t *testing.T) {
		_, err := shwap.SharesFromProto([]*pb.Share{nil})
		require.Error(t, err)
	})

	t.Run("SampleFromProto/nil share field", func(t *testing.T) {
		_, err := shwap.SampleFromProto(&pb.Sample{Share: nil})
		require.Error(t, err)
	})

	t.Run("RowFromProto/nil", func(t *testing.T) {
		_, err := shwap.RowFromProto(nil)
		require.Error(t, err)
	})

	t.Run("RowFromProto/nil share inside", func(t *testing.T) {
		_, err := shwap.RowFromProto(&pb.Row{SharesHalf: []*pb.Share{nil}})
		require.Error(t, err)
	})

	t.Run("RowNamespaceDataFromProto/nil", func(t *testing.T) {
		_, err := shwap.RowNamespaceDataFromProto(nil)
		require.Error(t, err)
	})

	t.Run("RangeNamespaceDataFromProto/nil", func(t *testing.T) {
		_, err := shwap.RangeNamespaceDataFromProto(nil)
		require.Error(t, err)
	})
}

// TestSampleReadFromRejectsNilShare simulates what shrex.Client.Get does after
// reading bytes from a malicious peer: serde.Read succeeds because pb.Sample
// allows the Share field to be unset, but SampleFromProto must reject it. If
// this regresses, the client will accept the malformed Sample and panic later
// inside Verify().
func TestSampleReadFromRejectsNilShare(t *testing.T) {
	var buf bytes.Buffer
	_, err := serde.Write(&buf, &pb.Sample{Share: nil})
	require.NoError(t, err)

	var s shwap.Sample
	_, err = s.ReadFrom(&buf)
	require.Error(t, err)
}

// TestZeroShareDoesNotEscapeDecoders ensures that no decoder hands out a
// zero-value libshare.Share (data == nil). A zero share panics on Namespace()
// and ToBytes(), so it must never be produced as a "success" value.
func TestZeroShareDoesNotEscapeDecoders(t *testing.T) {
	t.Run("ShareFromProto returns error, never a zero share", func(t *testing.T) {
		sh, err := shwap.ShareFromProto(nil)
		require.Error(t, err)
		require.Equal(t, libshare.Share{}, sh)
	})
}
