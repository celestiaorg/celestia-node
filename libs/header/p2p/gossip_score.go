package p2p

import (
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// GossibSubScore provides a set of recommended parameters for header GossipSub topic, a.k.a
// HeaderSub.
var GossibSubScore = pubsub.TopicScoreParams{
	// expected > 1 tx/second
	TopicWeight: 0.1, // max cap is 5, single invalid message is -100

	// 1 tick per second, maxes at 1 hour
	TimeInMeshWeight:  0.0002778, // ~1/3600
	TimeInMeshQuantum: time.Second,
	TimeInMeshCap:     1,

	// deliveries decay after 1 hour, cap at 100 blocks
	FirstMessageDeliveriesWeight: 5, // max value is 500
	FirstMessageDeliveriesDecay:  pubsub.ScoreParameterDecay(time.Hour),
	FirstMessageDeliveriesCap:    100, // 100 blocks in an hour

	// invalid messages decay after 1 hour
	InvalidMessageDeliveriesWeight: -1000,
	InvalidMessageDeliveriesDecay:  pubsub.ScoreParameterDecay(time.Hour),

	// Mesh Delivery Failure is currently turned off for messages
	// This is on purpose as the network is still too small, which results in
	// asymmetries and potential unmeshing from negative scores.
}
