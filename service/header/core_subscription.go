package header

import (
	"context"
	"fmt"

	"github.com/tendermint/tendermint/types"
)

type coreSubscription struct {
	coreEx *CoreExchange

	sub <-chan *types.Block
}

func newCoreSubscription(coreEx *CoreExchange) (*coreSubscription, error) {
	sub, err := coreEx.fetcher.SubscribeNewBlockEvent(context.Background())
	if err != nil {
		return nil, err
	}

	return &coreSubscription{
		coreEx: coreEx,
		sub: sub,
	}, nil
}

func (cs *coreSubscription) NextHeader(ctx context.Context) (*ExtendedHeader, error) {
	select {
	case <-ctx.Done():
		return nil, nil
	case newBlock, ok := <- cs.sub:
		if !ok {
			return nil, fmt.Errorf("subscription closed")
		}
		return cs.coreEx.generateExtendedHeaderFromBlock(newBlock)
	}
}

func (cs *coreSubscription) Cancel() error {
	return cs.coreEx.fetcher.UnsubscribeNewBlockEvent(context.Background())
}
