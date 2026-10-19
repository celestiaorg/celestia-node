package canary

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	libhead "github.com/celestiaorg/go-header"

	"github.com/celestiaorg/celestia-node/header"
)

// HeaderReader has header reads only: the canary never retrieves shares itself,
// so all sampling evidence comes from the node.
type HeaderReader interface {
	NetworkHead(context.Context) (*header.ExtendedHeader, error)
	// LocalHead is the node's own stored head: what the syncer has persisted.
	LocalHead(context.Context) (*header.ExtendedHeader, error)
	Tail(context.Context) (*header.ExtendedHeader, error)
	GetByHeight(context.Context, uint64) (*header.ExtendedHeader, error)
	GetByHash(context.Context, libhead.Hash) (*header.ExtendedHeader, error)
	GetRangeByHeight(context.Context, *header.ExtendedHeader, uint64) ([]*header.ExtendedHeader, error)
}

// LocalSuccessors reads the count headers after tail from the node's store (the
// range RPC's upper bound is exclusive). GetByHeight at the network head can
// bypass the store, so every hash is read back locally.
func LocalSuccessors(
	ctx context.Context,
	r HeaderReader,
	tail *header.ExtendedHeader,
	count int,
	chain string,
) ([]*header.ExtendedHeader, error) {
	if count <= 0 || count > 10000 {
		return nil, errors.New("invalid successor count")
	}
	if err := validateHeader(tail, chain); err != nil {
		return nil, err
	}
	if tail.Height() > ^uint64(0)>>1-uint64(count)-1 {
		return nil, errors.New("successor height overflow")
	}
	stored, err := r.GetByHash(ctx, tail.Hash())
	if err != nil {
		return nil, err
	}
	if err = validateHeader(stored, chain); err != nil {
		return nil, err
	}
	if !bytes.Equal(stored.Hash(), tail.Hash()) {
		return nil, errors.New("stored anchor mismatch")
	}
	var hs []*header.ExtendedHeader
	prev := tail
	for len(hs) < count {
		n := min(uint64(count-len(hs)), libhead.MaxRangeRequestSize)
		batch, err := r.GetRangeByHeight(ctx, prev, prev.Height()+n+1)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 || uint64(len(batch)) > n {
			return nil, fmt.Errorf("invalid native range length %d", len(batch))
		}
		for _, h := range batch {
			if err = validateHeader(h, chain); err != nil {
				return nil, err
			}
			if h.Height() != prev.Height()+1 {
				return nil, errors.New("non-adjacent height")
			}
			if err = prev.Verify(h); err != nil {
				return nil, err
			}
			stored, err = r.GetByHash(ctx, h.Hash())
			if err != nil {
				return nil, err
			}
			if err = validateHeader(stored, chain); err != nil {
				return nil, err
			}
			if !bytes.Equal(stored.Hash(), h.Hash()) {
				return nil, errors.New("stored successor mismatch")
			}
			prev = h
			hs = append(hs, h)
		}
	}
	return hs, nil
}

// ValidateHeadTail compares timestamps rather than estimated block counts. The
// storage window is measured back from the network head; the head's recency and
// clock skew are checked separately.
func ValidateHeadTail(
	head, tail *header.ExtendedHeader,
	chain string,
	now time.Time,
	window, recency, uncertainty time.Duration,
) error {
	if err := validateHeader(head, chain); err != nil {
		return err
	}
	if err := validateHeader(tail, chain); err != nil {
		return err
	}
	if window <= 0 || recency <= 0 || uncertainty < 0 {
		return errors.New("invalid window bounds")
	}
	if head.Height() < tail.Height() || head.Time().Before(tail.Time()) {
		return errors.New("inverted head/tail")
	}
	if head.Time().After(now.Add(uncertainty)) {
		return errors.New("future network head")
	}
	if now.Sub(head.Time()) > recency {
		return errors.New("stale network head")
	}
	if head.Time().Sub(tail.Time()) > window {
		return errors.New("tail outside storage window")
	}
	return nil
}

// headTailEvidence records a rejected head/tail pair. The tail age is what an
// alert reader needs first: by how much a tail overshoots the storage window.
func headTailEvidence(reason error, head, tail *header.ExtendedHeader, windowSeconds int64) map[string]any {
	ev := map[string]any{evidenceReason: reason.Error(), "storage_window_seconds": windowSeconds}
	if head != nil && tail != nil {
		ev["head_height"], ev["head_time"] = head.Height(), head.Time()
		ev["tail_height"], ev["tail_time"] = tail.Height(), tail.Time()
		ev["tail_age_seconds"] = head.Time().Sub(tail.Time()).Seconds()
	}
	return ev
}

func validateHeader(h *header.ExtendedHeader, chain string) error {
	if h == nil || h.Commit == nil || h.ValidatorSet == nil || h.DAH == nil {
		return errors.New("incomplete native header")
	}
	if h.ChainID() != chain {
		return errors.New("header chain mismatch")
	}
	return h.Validate()
}
