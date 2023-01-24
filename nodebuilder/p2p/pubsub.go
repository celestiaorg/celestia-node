package p2p

import (
	"context"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/protocol"
	"go.uber.org/fx"
	"golang.org/x/crypto/blake2b"
)

// PubSub provides a constructor for PubSub protocol with GossipSub routing.
func PubSub(cfg Config, params pubSubParams) (*pubsub.PubSub, error) {
	fpeers, err := cfg.mutualPeers()
	if err != nil {
		return nil, err
	}

	// TODO(@Wondertan) for PubSub options:
	//  * Hash-based MsgId function.
	//  * Validate default peer scoring params for our use-case.
	//  * Strict subscription filter
	//  * For different network types(mainnet/testnet/devnet) we should have different network topic
	// names.  * Hardcode positive score for bootstrap peers
	//  * Bootstrappers should only gossip and PX
	//  * Peers should trust boostrappers, so peerscore for them should always be high.
	opts := []pubsub.Option{
		pubsub.WithPeerExchange(cfg.PeerExchange || cfg.Bootstrapper),
		pubsub.WithDirectPeers(fpeers),
		pubsub.WithMessageIdFn(hashMsgID),
		// specifying sub protocol helps to avoid conflicts with
		// floodsub(because gossipsub supports floodsub protocol by default).
		pubsub.WithGossipSubProtocols([]protocol.ID{pubsub.GossipSubID_v11}, pubsub.GossipSubDefaultFeatures),
	}

	return pubsub.NewGossipSub(
		params.Ctx,
		params.Host,
		opts...,
	)
}

func hashMsgID(m *pubsub_pb.Message) string {
	hash := blake2b.Sum256(m.Data)
	return string(hash[:])
}

type pubSubParams struct {
	fx.In

	Ctx  context.Context
	Host host.Host
}
