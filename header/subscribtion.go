package header

import libhead "github.com/celestiaorg/celestia-node/libs/header"

//go:generate mockgen -destination=mocks/subscription.go -package=mocks . Subscription
type Subscription = libhead.Subscription[*ExtendedHeader]
