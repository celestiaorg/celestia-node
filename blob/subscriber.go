package blob

import (
	"context"
	"errors"
	"fmt"

	libshare "github.com/celestiaorg/go-square/v2/share"

	"github.com/celestiaorg/celestia-node/header"
)

type (
	// blobGetter is a function type that retrieves blobs from a given header
	// for the specified namespaces.
	blobGetter func(context.Context, *header.ExtendedHeader, []libshare.Namespace) ([]*Blob, error)
	// headerSub is a function type that subscribes to new headers.
	headerSub func(ctx context.Context) (<-chan *header.ExtendedHeader, error)
)

// SubscriptionResponse contains the blobs and the height at which they were included.
// If the Blobs slice is empty, it means that no blobs were included at the given height.
type SubscriptionResponse struct {
	Blobs  []*Blob
	Height uint64
}

// subscriber manages blob subscriptions by monitoring new headers and
// retrieving relevant blob data for specified namespaces.
type subscriber struct {
	ctx context.Context

	getter    blobGetter
	headerSub headerSub
}

func newSubscriber(getter blobGetter, sub headerSub) *subscriber {
	return &subscriber{
		getter:    getter,
		headerSub: sub,
	}
}

func (s *subscriber) Start(ctx context.Context) error {
	s.ctx = ctx
	return nil
}

// subscribe creates a subscription for blobs in the specified namespace.
// It returns a channel that will receive SubscriptionResponse messages
// whenever new blobs are detected for the given namespace.
func (s *subscriber) subscribe(ctx context.Context, ns libshare.Namespace) (<-chan *SubscriptionResponse, error) {
	if s.ctx == nil {
		if s.ctx == nil {
			return nil, fmt.Errorf("subscriber has not been started")
		}
	}

	headerCh, err := s.headerSub(ctx)
	if err != nil {
		return nil, err
	}

	blobCh := make(chan *SubscriptionResponse, 16)
	go s.onHeadersReceived(ctx, headerCh, blobCh, ns)
	return blobCh, nil
}

// onHeadersReceived is the subscription loop that processes incoming headers
// and retrieves blob data for the specified namespace.
func (s *subscriber) onHeadersReceived(
	ctx context.Context,
	headerCh <-chan *header.ExtendedHeader,
	blobCh chan *SubscriptionResponse,
	ns libshare.Namespace,
) {
	defer close(blobCh)

	var (
		hdr *header.ExtendedHeader
		ok  bool
	)

	for {
		select {
		case <-ctx.Done():
			log.Debugw("blobsub: canceling subscription due to user ctx closing", "namespace", ns.ID())
			return
		case <-s.ctx.Done():
			log.Debugw("blobsub: canceling subscription. subscriber is stopped", "namespace", ns.ID())
			return
		case hdr, ok = <-headerCh:
			if !ok {
				log.Errorw("blobsub: header channel closed for subscription", "namespace", ns.ID())
				return
			}
			break
		}

		// close subscription before buffer overflows
		// TODO: not sure about this. Consider removing it.
		if len(blobCh) == cap(blobCh) {
			log.Debugw("blobsub: canceling subscription due to buffer overflow from slow reader", "namespace", ns.ID())
			return
		}

		blobs, err := s.retrieveHeader(ctx, hdr, ns)
		if err != nil && !errors.Is(err, ErrBlobNotFound) {
			log.Debugw("blobsub: error retrieving header", "namespace", ns.ID(), "err", err)
			continue
		}

		select {
		case <-ctx.Done():
			log.Debugw("blobsub: pending response canceled due to user ctx closing", "namespace", ns.ID())
			return
		case blobCh <- &SubscriptionResponse{Blobs: blobs, Height: hdr.Height()}:
		}
	}
}

// retrieveHeader attempts to retrieve blobs from the given header for the
// specified namespace.
func (s *subscriber) retrieveHeader(
	ctx context.Context,
	hdr *header.ExtendedHeader,
	ns libshare.Namespace,
) ([]*Blob, error) {
	for {
		// No need to verify on ErrBlobNotFound, since getAll returns both empty values.
		blobs, err := s.getter(ctx, hdr, []libshare.Namespace{ns})
		if err == nil {
			return blobs, nil
		}

		switch {
		case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
			return nil, err
		case s.ctx.Err() != nil:
			return nil, s.ctx.Err()
		default:
			// retry in case of internal error
			log.Debugw("blobsub: error getting blobs", "namespace", ns.ID(), "err", err)
			continue
		}
	}
}
