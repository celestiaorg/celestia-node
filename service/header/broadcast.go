package header

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// broadcaster encompasses the ability to publish new ExtendedHeaders
// to the ExtendedHeader topic.
type broadcaster struct {
	topic *pubsub.Topic
}

func newBroadcaster(topic *pubsub.Topic) *broadcaster {
	return &broadcaster{
		topic: topic,
	}
}

// Broadcast broadcasts the given ExtendedHeader to the topic.
func (b *broadcaster) Broadcast(ctx context.Context, header *ExtendedHeader) error {
	bin, err := header.MarshalBinary()
	if err != nil {
		return err
	}
	return b.topic.Publish(ctx, bin)
}
