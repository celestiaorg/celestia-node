package shwap

import (
	"bytes"
	"github.com/celestiaorg/celestia-node/share/sharetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNewRangeNamespaceDataID(t *testing.T) {
	edsSize := 16
	ns := sharetest.RandV0Namespace()

	id, err := NewSampleID(1, 1, 1, edsSize)
	require.NoError(t, err)

	rngid, err := NewRangeNamespaceDataID(id, ns, 5, true, edsSize)
	require.NoError(t, err)

	bin, err := rngid.MarshalBinary()
	require.NoError(t, err)

	rngidOut, err := RangeNamespaceDataIDFromBinary(bin)
	require.NoError(t, err)
	assert.EqualValues(t, rngid, rngidOut)

	err = rngid.Validate()
	require.NoError(t, err)
}

func TestRangeNamespaceDataIDReaderWriter(t *testing.T) {
	edsSize := 32
	ns := sharetest.RandV0Namespace()

	id, err := NewSampleID(1, 2, 3, edsSize)
	require.NoError(t, err)

	rngid, err := NewRangeNamespaceDataID(id, ns, 10, false, edsSize)
	require.NoError(t, err)

	buf := bytes.NewBuffer(nil)
	n, err := rngid.WriteTo(buf)
	require.NoError(t, err)
	require.Equal(t, int64(RangeNamespaceDataIDSize), n)

	rngidOut := RangeNamespaceDataID{}
	n, err = rngidOut.ReadFrom(buf)
	require.NoError(t, err)
	require.Equal(t, int64(RangeNamespaceDataIDSize), n)
	require.EqualValues(t, rngid, rngidOut)

	err = rngidOut.Validate()
	require.NoError(t, err)
}
