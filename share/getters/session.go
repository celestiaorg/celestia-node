package getters

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/ipfs/go-blockservice"
)

// ses is a struct that can optionally be passed by context to the share.Getter methods using
// WithSession to indicate that a blockservice session should be created.
type ses struct {
	sync.Mutex
	atomic.Pointer[blockservice.Session]
}

// WithSession stores an empty ses in the context, indicating that a blockservice session should be
// created.
func WithSession(ctx context.Context) context.Context {
	return context.WithValue(ctx, ses{}, &ses{})
}

func getGetter(ctx context.Context, service blockservice.BlockService) blockservice.BlockGetter {
	s, ok := ctx.Value(ses{}).(*ses)
	if !ok {
		return service
	}

	val := s.Load()
	if val != nil {
		return val
	}

	s.Lock()
	defer s.Unlock()
	val = s.Load()
	if val == nil {
		val = blockservice.NewSession(ctx, service)
		s.Store(val)
	}
	return val
}
