package header

import "sync"

// ranges keeps non-overlapping and non-adjacent header ranges which are used to cache headers (in ascending order).
// This prevents unnecessary / duplicate network requests for additional headers during sync.
type ranges struct {
	ranges []*_range
	lk     sync.Mutex // no need for RWMutex as there is only one reader
}

// Head returns the highest ExtendedHeader in all ranges if any.
func (rs *ranges) Head() *ExtendedHeader {
	rs.lk.Lock()
	defer rs.lk.Unlock()

	ln := len(rs.ranges)
	if ln == 0 {
		return nil
	}

	head := rs.ranges[ln-1]
	return head.Head()
}

// Add appends the new ExtendedHeader to existing range or starts a new one.
// It starts a new one if the new ExtendedHeader is not adjacent to any of existing ranges.
func (rs *ranges) Add(h *ExtendedHeader) {
	head := rs.Head()

	// short-circuit if header is from the past
	if head != nil && head.Height >= h.Height {
		// TODO(@Wondertan): Technically, we can still apply the header:
		//  * Headers here are verified, so we can trust them
		//  * PubSub does not guarantee the ordering of msgs
		//    * So there might be a case where ordering is broken
		//    * Even considering the delay(block time) with which new headers are generated
		//    * But rarely
		//  Would be still nice to implement
		log.Warnf("rcvd headers in wrong order")
		return
	}

	rs.lk.Lock()
	defer rs.lk.Unlock()

	// if the new header is adjacent to head
	if head != nil && h.Height == head.Height+1 {
		// append it to the last known range
		rs.ranges[len(rs.ranges)-1].Append(h)
	} else {
		// otherwise, start a new range
		rs.ranges = append(rs.ranges, newRange(h))

		// it is possible to miss a header or few from PubSub, due to quick disconnects or sleep
		// once we start rcving them again we save those in new range
		// so 'Syncer.getHeaders' can fetch what was missed
	}
}

// BackWithin searches for a range within a given height span (start:end].
func (rs *ranges) BackWithin(start, end uint64) (*_range, bool) {
	r, ok := rs.Back()
	if !ok {
		return nil, false
	}

	if r.Start >= start && r.Start < end {
		return r, true
	}

	return nil, false
}

// Back provides a first non-empty range, while cleaning up empty ones.
func (rs *ranges) Back() (*_range, bool) {
	rs.lk.Lock()
	defer rs.lk.Unlock()

	for {
		if len(rs.ranges) == 0 {
			return nil, false
		}

		out := rs.ranges[0]
		if !out.Empty() {
			return out, true
		}

		rs.ranges = rs.ranges[1:]
	}
}

type _range struct {
	Start   uint64
	lk      sync.Mutex // no need for RWMutex as there is only one reader
	headers []*ExtendedHeader
}

func newRange(h *ExtendedHeader) *_range {
	return &_range{
		Start:   uint64(h.Height),
		headers: []*ExtendedHeader{h},
	}
}

// Append appends new headers.
func (r *_range) Append(h ...*ExtendedHeader) {
	r.lk.Lock()
	r.headers = append(r.headers, h...)
	r.lk.Unlock()
}

// Empty reports if range is empty.
func (r *_range) Empty() bool {
	r.lk.Lock()
	defer r.lk.Unlock()
	return len(r.headers) == 0
}

// Head reports the head of range if any.
func (r *_range) Head() *ExtendedHeader {
	r.lk.Lock()
	defer r.lk.Unlock()
	ln := len(r.headers)
	if ln == 0 {
		return nil
	}
	return r.headers[ln-1]
}

// Before cuts off all the headers before height 'end' and returns them.
func (r *_range) Before(end uint64) []*ExtendedHeader {
	r.lk.Lock()
	defer r.lk.Unlock()

	amnt := uint64(len(r.headers))
	if r.Start+amnt > end {
		amnt = end - r.Start
	}

	out := r.headers[:amnt]
	r.headers = r.headers[amnt:]
	if len(r.headers) != 0 {
		r.Start = uint64(r.headers[0].Height)
	}
	return out
}
