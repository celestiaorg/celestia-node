package file

import (
	"sync"

	"github.com/klauspost/reedsolomon"
)

var codec Codec

func init() {
	codec = NewCodec()
}

type Codec interface {
	Encoder(len int) (reedsolomon.Encoder, error)
}

type codecCache struct {
	cache sync.Map
}

func NewCodec() Codec {
	return &codecCache{}
}

func (l *codecCache) Encoder(len int) (reedsolomon.Encoder, error) {
	enc, ok := l.cache.Load(len)
	if !ok {
		var err error
		enc, err = reedsolomon.New(len/2, len/2, reedsolomon.WithLeopardGF(true))
		if err != nil {
			return nil, err
		}
		l.cache.Store(len, enc)
	}
	return enc.(reedsolomon.Encoder), nil
}
