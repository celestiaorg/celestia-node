package p2p

import (
	"go.uber.org/fx"
)

type Config struct {
	// ListenAddresses - Addresses to listen to on local NIC.
	ListenAddresses []string
	// AnnounceAddresses - Addresses to be announced/advertised for peers to connect to
	AnnounceAddresses []string
	// NoAnnounceAddresses - Addresses the P2P subsystem may know about, but that should not be announced/advertised,
	// as undialable from WAN
	NoAnnounceAddresses []string
}

// Components collects all the components and services related to p2p.
func Components(cfg *Config) fx.Option {
	return fx.Options(
		fx.Provide(Host()),
		fx.Provide(AddrsFactory(cfg.AnnounceAddresses, cfg.NoAnnounceAddresses)),
		fx.Invoke(Listen(cfg.ListenAddresses)),
	)
}
