package fraud

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// topic holds pubsub topic and unmarshaler of corresponded Fraud Proof
type topic struct {
	topic     *pubsub.Topic
	unmarshal ProofUnmarshaler
}

// publish allows to publish Fraud Proofs to the network
func (t *topic) publish(ctx context.Context, data []byte) error {
	return t.topic.Publish(ctx, data)
}

// close removes unmarshaler and closes pubsub topic
func (t *topic) close() error {
	t.unmarshal = nil
	return t.topic.Close()
}
