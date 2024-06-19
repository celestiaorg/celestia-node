package file

import (
	"runtime"
	"sync"

	"github.com/celestiaorg/celestia-node/share"
)

// TODO: need better name
var memPools poolsMap

func init() {
	memPools = make(map[int]*memPool)
}

type poolsMap map[int]*memPool

type memPool struct {
	ods      *sync.Pool
	halfAxis *sync.Pool
}

func (m poolsMap) get(size int) *memPool {
	pool, ok := m[size]
	if !ok {
		pool = &memPool{
			ods:      newOdsPool(size),
			halfAxis: newHalfAxisPool(size),
		}
		m[size] = pool
	}
	return pool
}

func (m *memPool) putSquare(s *[][]share.Share) {
	m.ods.Put(s)
}

func (m *memPool) square() [][]share.Share {
	square := m.ods.Get().(*[][]share.Share)
	runtime.SetFinalizer(square, m.putSquare)
	return *square
}

func (m *memPool) putHalfAxis(buf *[]share.Share) {
	m.halfAxis.Put(buf)
}

func (m *memPool) getHalfAxis() []share.Share {
	half := m.halfAxis.Get().(*[]share.Share)
	runtime.SetFinalizer(half, m.putHalfAxis)
	return *half
}

func newOdsPool(size int) *sync.Pool {
	return &sync.Pool{
		New: func() interface{} {
			rows := make([][]share.Share, size)
			for i := range rows {
				if rows[i] == nil {
					rows[i] = newHalfAxis(size)
				}
			}
			return &rows
		},
	}
}

func newHalfAxisPool(size int) *sync.Pool {
	return &sync.Pool{
		New: func() interface{} {
			half := newHalfAxis(size)
			return &half
		},
	}
}

func newHalfAxis(size int) []share.Share {
	shares := make([]share.Share, size)
	for i := range shares {
		shares[i] = make([]byte, share.Size)
	}
	return shares
}
