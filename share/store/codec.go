package store

import (
	"sync"

	"github.com/klauspost/reedsolomon"
)

type Codec interface {
	Encoder(len int) (reedsolomon.Encoder, error)
}

type codec struct {
	encCache sync.Map
}

func NewCodec() Codec {
	return &codec{}
}

func (l *codec) Encoder(len int) (reedsolomon.Encoder, error) {
	enc, ok := l.encCache.Load(len)
	if !ok {
		var err error
		enc, err = reedsolomon.New(len/2, len/2, reedsolomon.WithLeopardGF(true))
		if err != nil {
			return nil, err
		}
		l.encCache.Store(len, enc)
	}
	return enc.(reedsolomon.Encoder), nil
}
