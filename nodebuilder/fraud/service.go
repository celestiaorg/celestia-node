package fraud

import "github.com/celestiaorg/celestia-node/fraud"

// Service encompasses the behavior necessary to subscribe and broadcast
// fraud proofs within the network.
type Service interface {
	fraud.Subscriber
	fraud.Broadcaster
	fraud.Getter
}
