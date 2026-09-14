package shwap_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

func TestNamespaceDataReadFromCapsRows(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	maxRows := share.MaxSquareSize

	// serialize a single valid row to replay as the attacker's payload.
	const odsSize = 8
	namespace := libshare.RandomNamespace()
	randEDS, _ := edstest.RandEDSWithNamespace(t, namespace, odsSize, odsSize)
	nd, err := eds.NamespaceData(ctx, &eds.Rsmt2D{ExtendedDataSquare: randEDS}, namespace)
	require.NoError(t, err)
	require.NotEmpty(t, nd)

	var row bytes.Buffer
	_, err = nd[0].WriteTo(&row)
	require.NoError(t, err)

	// one more row than a namespace can occupy in the largest possible ODS.
	var stream bytes.Buffer
	for range maxRows + 1 {
		stream.Write(row.Bytes())
	}

	var got shwap.NamespaceData
	_, err = got.ReadFrom(&stream)
	require.Error(t, err)
	require.Empty(t, got, "no rows must be committed when the cap is exceeded")
}

// TestNamespaceDataReadFromRoundTrip ensures a well-formed response of legitimate
// size still round-trips through WriteTo/ReadFrom unchanged.
func TestNamespaceDataReadFromRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	const odsSize = 8
	namespace := libshare.RandomNamespace()
	randEDS, _ := edstest.RandEDSWithNamespace(t, namespace, odsSize*odsSize/2, odsSize)
	nd, err := eds.NamespaceData(ctx, &eds.Rsmt2D{ExtendedDataSquare: randEDS}, namespace)
	require.NoError(t, err)
	require.NotEmpty(t, nd)

	var buf bytes.Buffer
	_, err = nd.WriteTo(&buf)
	require.NoError(t, err)

	var got shwap.NamespaceData
	_, err = got.ReadFrom(&buf)
	require.NoError(t, err)
	require.Equal(t, nd, got)
}
