package header

import (
	"context"
	"sync"
	"sync/atomic"
)

// heightSub provides a minimalistic mechanism to wait till header for a height becomes available.
type heightSub struct {
	height       uint64 // atomic
	heightReqsLk sync.Mutex
	heightReqs   map[uint64][]chan *ExtendedHeader
}

// newHeightSub instantiates new heightSub.
func newHeightSub() *heightSub {
	return &heightSub{
		heightReqs: make(map[uint64][]chan *ExtendedHeader),
	}
}

// Sub subscribes for a header of a given height.
// It can return both values as nil, which means a requested header was already provided
// and caller should get it elsewhere.
func (hs *heightSub) Sub(ctx context.Context, height uint64) (*ExtendedHeader, error) {
	if atomic.LoadUint64(&hs.height) >= height {
		return nil, nil
	}

	hs.heightReqsLk.Lock()
	if atomic.LoadUint64(&hs.height) >= height {
		// This is a rare case we have to account for.
		// The lock above can park a goroutine long enough for hs.height to change for a requested height,
		// leaving the request never fulfilled and the goroutine deadlocked.
		return nil, nil
	}
	resp := make(chan *ExtendedHeader, 1)
	hs.heightReqs[height] = append(hs.heightReqs[height], resp)
	hs.heightReqsLk.Unlock()

	select {
	case resp := <-resp:
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Pub processes all the outstanding subscriptions matching the given headers.
// Pub is safe to be called from one goroutine.
func (hs *heightSub) Pub(headers ...*ExtendedHeader) {
	height := atomic.LoadUint64(&hs.height)
	from, to := uint64(headers[0].Height), uint64(headers[len(headers)-1].Height)
	if height != 0 && height+1 != from {
		log.Warnf("PLEASE REPORT THE BUG: headers given to the heightSub are in the wrong order")
		return
	}
	atomic.StoreUint64(&hs.height, to)

	hs.heightReqsLk.Lock()
	defer hs.heightReqsLk.Unlock()

	// instead of looping over each header in 'headers', we can loop over each request
	// which will drastically decrease idle iterations, as there will be lesser requests than the headers
	for height, reqs := range hs.heightReqs {
		// then we look if any of the requests match the given range of headers
		if height >= from && height <= to {
			// and if so, calculate its position and fulfill requests
			h := headers[height-from]
			for _, req := range reqs {
				req <- h // reqs must always be buffered, so this won't block
			}
			delete(hs.heightReqs, height)
		}
	}
}
