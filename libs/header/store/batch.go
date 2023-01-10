package store

import (
	"sync"

	"github.com/celestiaorg/celestia-node/libs/header"
)

// batch keeps an adjacent range of headers and loosely mimics the Store
// interface. NOTE: Can fully implement Store for a use case.
//
// It keeps a mapping 'height -> header' and 'hash -> height'
// unlike the Store which keeps 'hash -> header' and 'height -> hash'.
// The approach simplifies implementation for the batch and
// makes it better optimized for the GetByHeight case which is what we need.
type batch[H header.Header] struct {
	lk      sync.RWMutex
	heights map[string]uint64
	headers []H
}

// newBatch creates the batch with the given pre-allocated size.
func newBatch[H header.Header](size int) *batch[H] {
	return &batch[H]{
		heights: make(map[string]uint64, size),
		headers: make([]H, 0, size),
	}
}

// Len gives current length of the batch.
func (b *batch[H]) Len() int {
	b.lk.RLock()
	defer b.lk.RUnlock()
	return len(b.headers)
}

// GetAll returns a slice of all the headers in the batch.
func (b *batch[H]) GetAll() []H {
	b.lk.RLock()
	defer b.lk.RUnlock()
	return b.headers
}

// Get returns a header by its hash.
func (b *batch[H]) Get(hash header.Hash) H {
	b.lk.RLock()
	defer b.lk.RUnlock()
	height, ok := b.heights[hash.String()]
	if !ok {
		var zero H
		return zero
	}

	return b.getByHeight(height)
}

// GetByHeight returns a header by its height.
func (b *batch[H]) GetByHeight(height uint64) H {
	b.lk.RLock()
	defer b.lk.RUnlock()
	return b.getByHeight(height)
}

func (b *batch[H]) getByHeight(height uint64) H {
	var (
		ln   = uint64(len(b.headers))
		zero H
	)
	if ln == 0 {
		return zero
	}

	head := uint64(b.headers[ln-1].Height())
	base := head - ln
	if height > head || height <= base {
		return zero
	}

	return b.headers[height-base-1]
}

// Append appends new headers to the batch.
func (b *batch[H]) Append(headers ...H) {
	b.lk.Lock()
	defer b.lk.Unlock()
	for _, h := range headers {
		b.headers = append(b.headers, h)
		b.heights[h.Hash().String()] = uint64(h.Height())
	}
}

// Has checks whether header by the hash is present in the batch.
func (b *batch[H]) Has(hash header.Hash) bool {
	b.lk.RLock()
	defer b.lk.RUnlock()
	_, ok := b.heights[hash.String()]
	return ok
}

// Reset cleans references to batched headers.
func (b *batch[H]) Reset() {
	b.lk.Lock()
	defer b.lk.Unlock()
	b.headers = b.headers[:0]
	for k := range b.heights {
		delete(b.heights, k)
	}
}
