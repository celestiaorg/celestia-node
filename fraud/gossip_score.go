package fraud

import (
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// GossibSubScore provides a set of recommended parameters for header GossipSub topic, a.k.a HeaderSub.
// TODO(@Wondertan): We should disable mesh on publish for this topic to minimize chances of censoring FPs
//  by eclipsing nodes producing them
var GossibSubScore = &pubsub.TopicScoreParams{
	// expected > 1 tx/second
	TopicWeight: 0.1, // max cap is 5, single invalid message is -100

	// 1 tick per second, maxes at 1 hour
	TimeInMeshWeight:  0.0002778, // ~1/3600
	TimeInMeshQuantum: time.Second,
	TimeInMeshCap:     1,

	// messages in such topics should almost never exist, but very valuable if happens
	// so giving max weight
	FirstMessageDeliveriesWeight: 50,
	FirstMessageDeliveriesDecay:  pubsub.ScoreParameterDecay(10 * time.Hour),
	// no cap, if the peer is giving us *valid* FPs, just keep increasing peer's score with no limit
	// again, this is such a rare case to happen, but if it happens, we should prefer the peer who
	// gave it to us
	FirstMessageDeliveriesCap:    0,

	// we don't really need this, as we block list peers who give us a bad message,
	// so disabled
	InvalidMessageDeliveriesWeight: 0,

	// Mesh Delivery Scoring is turned off as well.
}
