package utils

import (
	"errors"
	"io"
)

type readerAtWrapper struct {
	readerAt   io.ReaderAt
	readOffset int64
}

// NewReaderAtWrapper creates a new io.Reader wrapper around an io.ReaderAt instance.
func NewReaderAtWrapper(readerAt io.ReaderAt) io.Reader {
	return &readerAtWrapper{readerAt: readerAt}
}

// Read implements the io.Reader interface.
func (w *readerAtWrapper) Read(p []byte) (n int, err error) {
	if w.readerAt == nil {
		return 0, errors.New("readerAtWrapper: readerAt is nil")
	}

	n, err = w.readerAt.ReadAt(p, w.readOffset)
	w.readOffset += int64(n)

	if err == io.EOF {
		return n, err
	}

	if err != nil {
		return n, errors.New("readerAtWrapper: error reading from the underlying ReaderAt")
	}

	return n, nil
}
