package header

import (
	"context"

	"github.com/ipfs/go-blockservice"
	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-node/libs/header"
	"github.com/celestiaorg/celestia-node/libs/header/p2p"
	"github.com/celestiaorg/celestia-node/libs/header/sync"
)

type Subscriber = header.Subscriber[*ExtendedHeader]
type Subscription = header.Subscription[*ExtendedHeader]
type Broadcaster = header.Broadcaster[*ExtendedHeader]
type Exchange = header.Exchange[*ExtendedHeader]
type Store = header.Store[*ExtendedHeader]
type Getter = header.Getter[*ExtendedHeader]
type Head = header.Head[*ExtendedHeader]
type Syncer = sync.Syncer[*ExtendedHeader]
type ExchangeServer = p2p.ExchangeServer[*ExtendedHeader]

// ConstructFn aliases a function that creates a Header.
type ConstructFn = func(
	context.Context,
	*types.Block,
	*types.Commit,
	*types.ValidatorSet,
	blockservice.BlockService,
) (*ExtendedHeader, error)
