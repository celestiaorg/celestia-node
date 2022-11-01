package share

import "context"

// Discovereable defines interface for peer discovery.
type Discoverable interface {
	// Parameterizable allows the implemeters of Discoverable to be configurable/parameterizable
	Parameterizable

	// EnsurePeers ensures we always have "number" of connected peers.
	// It starts peer discovery every "discovery interval (s)" until peer cache reaches "peers count".
	// Discovery is restarted if any previously connected peers disconnect.
	EnsurePeers(context.Context)

	// Advertise is a utility function that persistently advertises a service through an Advertiser.
	Advertise(context.Context)
}
