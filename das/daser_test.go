package das

import (
	"context"
	"sync"
	"testing"

	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/celestia-node/service/share"
)

func TestDASer(t *testing.T) {
	shareServ, dah := share.RandServiceWithSquare(t, 16)

	randHeader := header.RandExtendedHeader(t)
	randHeader.DataHash = dah.Hash()
	randHeader.DAH = dah

	sub := &mockHeaderSub{
		headers: []*header.ExtendedHeader{randHeader},
	}

	daser := NewDASer(shareServ, sub)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		daser.sampling(context.Background(), sub)
		wg.Done()
	}(wg)
	wg.Wait()
}

type mockHeaderSub struct {
	headers []*header.ExtendedHeader
}

func (mhs *mockHeaderSub) Subscribe() (header.Subscription, error) {
	return mhs, nil
}

func (mhs *mockHeaderSub) NextHeader(ctx context.Context) (*header.ExtendedHeader, error) {
	defer func() {
		mhs.headers = make([]*header.ExtendedHeader, 0)
	}()
	if len(mhs.headers) == 0 {
		return nil, context.Canceled
	}
	return mhs.headers[0], nil
}

func (mhs *mockHeaderSub) Cancel() {}
