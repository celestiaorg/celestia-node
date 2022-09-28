package fraud

import "github.com/celestiaorg/celestia-node/fraud"

// Module encompasses the behavior necessary to subscribe and broadcast
// fraud proofs within the network.
type Module interface {
	fraud.Subscriber
	fraud.Broadcaster
	fraud.Getter
}
