package core

import "github.com/celestiaorg/celestia-node/nodebuilder/pruner"

type ListenerOption func(*Listener)
type ExchangeOption func(*Exchange)

func WithListenerStoragePruner(pruner *pruner.StoragePruner) ListenerOption {
	return func(l *Listener) {
		l.pruner = pruner
	}
}

func WithExchangeStoragePruner(pruner *pruner.StoragePruner) ExchangeOption {
	return func(e *Exchange) {
		e.pruner = pruner
	}
}
