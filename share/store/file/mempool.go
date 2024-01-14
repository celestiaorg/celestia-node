package file

import (
	"github.com/celestiaorg/celestia-node/share"
	"sync"
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

// TODO: test me
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

func (m *memPool) putOds(ods [][]share.Share) {
	m.ods.Put(ods)
}

func (m *memPool) getOds() [][]share.Share {
	return m.ods.Get().([][]share.Share)
}

func (m *memPool) putHalfAxis(buf []byte) {
	m.halfAxis.Put(buf)
}

func (m *memPool) getHalfAxis() []byte {
	return m.halfAxis.Get().([]byte)
}

func newOdsPool(size int) *sync.Pool {
	return &sync.Pool{
		New: func() interface{} {
			shrs := make([][]share.Share, size)
			for i := range shrs {
				if shrs[i] == nil {
					shrs[i] = make([]share.Share, size)
					for j := range shrs[i] {
						shrs[i][j] = make(share.Share, share.Size)
					}
				}
			}
			return shrs
		},
	}
}

func newHalfAxisPool(size int) *sync.Pool {
	return &sync.Pool{
		New: func() interface{} {
			buf := make([]byte, size*share.Size)
			return buf
		},
	}
}
