package eds

import (
	"bytes"
	crand "crypto/rand"
	"errors"
	"io"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewBufferedReaderMany(t *testing.T) {
	// create io.Writer that write random data
	for i := 0; i < 10000; i++ {
		TestNewBufferedReader(t)
	}
}

func TestNewBufferedReader(t *testing.T) {
	// create io.Writer that write random data
	size := 200
	randAmount := size + rand.Intn(size)
	randBytes := make([]byte, randAmount)
	_, err := crand.Read(randBytes)
	require.NoError(t, err)

	// randBytes := bytes.Repeat([]byte("1234567890"), 10)

	reader := NewBufferedReader(randMinWriter{bytes.NewReader(randBytes)})
	readBytes, err := readWithRandomBuffer(reader, size/10)
	require.NoError(t, err)
	require.Equal(t, randBytes, readBytes)
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

type randMinWriter struct {
	*bytes.Reader
}

func (lwt randMinWriter) WriteTo(writer io.Writer, limit int) (int, error) {
	var amount int
	for amount < limit {
		bufLn := limit
		if bufLn > 1 {
			bufLn = rand.Intn(limit-1) + 1
		}
		buf := make([]byte, bufLn)
		n, err := lwt.Read(buf)
		if err != nil {
			return amount, err
		}
		n, err = writer.Write(buf[:n])
		amount += n
		if err != nil {
			return amount, err
		}
		if n < bufLn {
			return amount, io.EOF
		}
	}
	return amount, nil
}
