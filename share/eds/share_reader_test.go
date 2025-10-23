package eds

import (
	"errors"
	"io"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v3/share"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
)

func TestShareReader(t *testing.T) {
	// create io.Writer that write random data
	odsSize := 16
	eds := edstest.RandEDS(t, odsSize)
	getShare := func(rowIdx, colIdx int) (libshare.Share, error) {
		rawShare := eds.GetCell(uint(rowIdx), uint(colIdx))
		sh, err := libshare.NewShare(rawShare)
		if err != nil {
			return libshare.Share{}, err
		}
		return *sh, nil
	}

	reader := NewShareReader(odsSize, getShare)
	readBytes, err := readWithRandomBuffer(reader, 1024)
	require.NoError(t, err)
	expected := make([]byte, 0, odsSize*odsSize*libshare.ShareSize)
	for _, share := range eds.FlattenedODS() {
		expected = append(expected, share...)
	}
	require.Len(t, readBytes, len(expected))
	require.Equal(t, expected, readBytes)
}

// testRandReader reads from reader with buffers of random sizes.
func readWithRandomBuffer(reader io.Reader, maxBufSize int) ([]byte, error) {
	// create buffer of random size
	data := make([]byte, 0, maxBufSize)
	for {
		bufSize := rand.Intn(maxBufSize-1) + 1
		buf := make([]byte, bufSize)
		n, err := reader.Read(buf)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		if n < bufSize {
			buf = buf[:n]
		}
		data = append(data, buf...)
		if errors.Is(err, io.EOF) {
			break
		}
	}
	return data, nil
}
