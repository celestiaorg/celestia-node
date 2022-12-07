package header

import (
	headerpkg "github.com/celestiaorg/celestia-node/pkg/header"
	"github.com/celestiaorg/celestia-node/pkg/header/p2p"
	"github.com/celestiaorg/celestia-node/pkg/header/sync"
)

type Subscriber = headerpkg.Subscriber[*ExtendedHeader]
type Subscription = headerpkg.Subscription[*ExtendedHeader]
type Broadcaster = headerpkg.Broadcaster[*ExtendedHeader]
type Exchange = headerpkg.Exchange[*ExtendedHeader]
type Store = headerpkg.Store[*ExtendedHeader]
type Getter = headerpkg.Getter[*ExtendedHeader]
type Head = headerpkg.Head[*ExtendedHeader]
type Syncer = sync.Syncer[*ExtendedHeader]
type ExchangeServer = p2p.ExchangeServer[*ExtendedHeader]
