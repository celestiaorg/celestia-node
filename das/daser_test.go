package das

import (
	"context"
	"sync"
	"testing"

	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/celestia-node/service/share"
)

func TestDASer(t *testing.T) {
	shareServ, dah := share.RandLightServiceWithSquare(t, 16)

	randHeader := header.RandExtendedHeader(t)
	randHeader.DataHash = dah.Hash()
	randHeader.DAH = dah

	sub := &header.DummySubscriber{
		Headers: []*header.ExtendedHeader{randHeader},
	}

	mockGet := new(mockGetter)

	daser := NewDASer(shareServ, sub, mockGet, nil) // TODO @renaynay

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		daser.sampleLatest(context.Background(), sub)
		wg.Done()
	}(wg)
	wg.Wait()
}

type mockGetter struct {

}

func (m mockGetter) GetByHeight(ctx context.Context, u uint64) (*header.ExtendedHeader, error) {
	panic("implement me")
}

