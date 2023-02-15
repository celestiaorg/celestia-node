package p2p

import (
	"context"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	hst "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"go.uber.org/fx"
	"golang.org/x/crypto/blake2b"
)

// pubSub provides a constructor for PubSub protocol with GossipSub routing.
func pubSub(cfg Config, params pubSubParams) (*pubsub.PubSub, error) {
	fpeers, err := cfg.mutualPeers()
	if err != nil {
		return nil, err
	}

	isBootstrapper := cfg.Bootstrapper
	bootstrappers := map[peer.ID]struct{}{}
	for _, b := range params.Bootstrappers {
		bootstrappers[b.ID] = struct{}{}
	}

	if isBootstrapper {
		// Turn off the mesh in bootstrappers as per:
		// https://github.com/libp2p/specs/blob/master/pubsub/gossipsub/gossipsub-v1.1.md#recommendations-for-network-operators
		pubsub.GossipSubD = 0
		pubsub.GossipSubDscore = 0
		pubsub.GossipSubDlo = 0
		pubsub.GossipSubDhi = 0
		pubsub.GossipSubDout = 0
		pubsub.GossipSubDlazy = 64
		pubsub.GossipSubGossipFactor = 0.25
		pubsub.GossipSubPruneBackoff = 5 * time.Minute
	}

	// TODO(@Wondertan) for PubSub options:
	//  * Validate and improve default peer scoring params
	//  * Strict subscription filter
	opts := []pubsub.Option{
		// Based on:
		//	* https://github.com/libp2p/specs/blob/master/pubsub/gossipsub/gossipsub-v1.1.md#peer-scoring
		//  * lotus
		//  * prysm
		pubsub.WithPeerScore(
			&pubsub.PeerScoreParams{
				AppSpecificScore: func(p peer.ID) float64 {
					// return a heavy positive score for bootstrappers so that we don't unilaterally prune
					// them and accept PX from them.
					// we don't do that in the bootstrappers themselves to avoid creating a closed mesh
					// between them (however we might want to consider doing just that)
					_, ok := bootstrappers[p]
					if ok && !isBootstrapper {
						return 2500
					}

					// TODO: we want to plug the application specific score to the node itself in order
					//       to provide feedback to the pubsub system based on observed behaviour
					return 0
				},
				AppSpecificWeight: 1,

				// This sets the IP colocation threshold to 5 peers before we apply penalties
				IPColocationFactorThreshold: 10,
				IPColocationFactorWeight:    -100,
				IPColocationFactorWhitelist: nil,

				BehaviourPenaltyThreshold: 6,
				BehaviourPenaltyWeight:    -10,
				BehaviourPenaltyDecay:     pubsub.ScoreParameterDecay(time.Hour),

				DecayInterval: pubsub.DefaultDecayInterval,
				DecayToZero:   pubsub.DefaultDecayToZero,

				// this retains *non-positive* scores for 6 hours
				RetainScore: 6 * time.Hour,

				Topics: map[string]*pubsub.TopicScoreParams{
				},
			},
			nil,
		),
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
	Host hst.Host
	Bootstrappers Bootstrappers
	Network Network
}
