package shwap

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRowStreamReader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	failure := errors.New("row read failed")
	r, err := newRowStreamReader(ctx, 3, func(i int) (RowNamespaceData, error) {
		calls++
		if i == 1 {
			return RowNamespaceData{}, failure
		}
		return RowNamespaceData{}, nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	var encoded bytes.Buffer
	_, err = (RowNamespaceData{}).WriteTo(&encoded)
	require.NoError(t, err)
	_, err = io.ReadFull(r, make([]byte, encoded.Len()))
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	_, err = r.Read(make([]byte, 1))
	require.ErrorIs(t, err, failure)
	_, err = r.Read(make([]byte, 1))
	require.ErrorIs(t, err, failure)
	require.Equal(t, 2, calls)

	cancel()
	_, err = newRowStreamReader(ctx, 1, func(int) (RowNamespaceData, error) {
		t.Fatal("read after cancellation")
		return RowNamespaceData{}, nil
	})
	require.ErrorIs(t, err, context.Canceled)
}

func TestRowStreamReaderUnexpectedEOF(t *testing.T) {
	r, err := newRowStreamReader(context.Background(), 2, func(i int) (RowNamespaceData, error) {
		if i == 1 {
			return RowNamespaceData{}, io.EOF
		}
		return RowNamespaceData{}, nil
	})
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, r)
	require.Error(t, err, "a missing row must not look like the end of the response")
}
