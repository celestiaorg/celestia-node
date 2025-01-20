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
	Encoder(ln int) (reedsolomon.Encoder, error)
}

type codecCache struct {
	cache sync.Map
}

func NewCodec() Codec {
	return &codecCache{}
}

func (l *codecCache) Encoder(ln int) (reedsolomon.Encoder, error) {
	enc, ok := l.cache.Load(ln)
	if !ok {
		var err error
		enc, err = reedsolomon.New(ln/2, ln/2, reedsolomon.WithLeopardGF(true))
		if err != nil {
			return nil, err
		}
		l.cache.Store(ln, enc)
	}
	return enc.(reedsolomon.Encoder), nil
}
