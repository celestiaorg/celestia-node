package p2p

import (
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// GossibSubScore provides a set of recommended parameters for header GossipSub topic, a.k.a HeaderSub.
var GossibSubScore = &pubsub.TopicScoreParams{
	// expected > 1 tx/second
	TopicWeight: 0.1, // max cap is 5, single invalid message is -100

	// 1 tick per second, maxes at 1 hour
	TimeInMeshWeight:  0.0002778, // ~1/3600
	TimeInMeshQuantum: time.Second,
	TimeInMeshCap:     1,

	// deliveries decay after 10min, cap at 100 tx
	FirstMessageDeliveriesWeight: 0.5, // max value is 50
	FirstMessageDeliveriesDecay:  pubsub.ScoreParameterDecay(10 * time.Minute),
	FirstMessageDeliveriesCap:    100, // 100 messages in 10 minutes

	// invalid messages decay after 1 hour
	InvalidMessageDeliveriesWeight: -1000,
	InvalidMessageDeliveriesDecay:  pubsub.ScoreParameterDecay(time.Hour),

	// Mesh Delivery Failure is currently turned off for messages
	// This is on purpose as the network is still too small, which results in
	// asymmetries and potential unmeshing from negative scores.
}
