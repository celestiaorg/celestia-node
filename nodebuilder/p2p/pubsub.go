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

	opts := []pubsub.Option{
		// TODO(@Wondertan) Validate and improve default peer scoring params
		// Сurrent parameters are based on:
		//	* https://github.com/libp2p/specs/blob/master/pubsub/gossipsub/gossipsub-v1.1.md#peer-scoring
		//  * lotus
		//  * prysm
		pubsub.WithPeerScore(
			peerScoreParams(isBootstrapper, params.Bootstrappers),
			peerScoreThresholds(),
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

func peerScoreParams(isBootstrapper bool, bootstrappers Bootstrappers) *pubsub.PeerScoreParams {
	bootstrapperSet := map[peer.ID]struct{}{}
	for _, b := range bootstrappers {
		bootstrapperSet[b.ID] = struct{}{}
	}

	// See https://github.com/libp2p/specs/blob/master/pubsub/gossipsub/gossipsub-v1.1.md#the-score-function
	return &pubsub.PeerScoreParams{
		AppSpecificScore: func(p peer.ID) float64 {
			// return a heavy positive score for bootstrappers so that we don't unilaterally prune
			// them and accept PX from them
			_, ok := bootstrapperSet[p]
			if ok && !isBootstrapper {
				return 2500
			}

			// TODO(@Wondertan):
			//  Plug the application specific score to the node itself in order
			//  to provide feedback to the pubsub system based on observed behaviour
			return 0
		},
		AppSpecificWeight: 1,

		// This sets the IP colocation threshold to 5 peers before we apply penalties
		// The aim is to protect the PubSub from naive bots collocated on the same machine/datacenter
		IPColocationFactorThreshold: 10,
		IPColocationFactorWeight:    -100,
		// TODO(@Wondertan): Make this configurable, e.g. we might have Testground bots for testing purposes
		IPColocationFactorWhitelist: nil,

		BehaviourPenaltyThreshold: 6,
		BehaviourPenaltyWeight:    -10,
		BehaviourPenaltyDecay:     pubsub.ScoreParameterDecay(time.Hour),

		// Scores should not only grow and this defines a decay function equal for each peer
		DecayInterval: pubsub.DefaultDecayInterval,
		DecayToZero:   pubsub.DefaultDecayToZero,

		// this retains *non-positive* scores for 6 hours
		RetainScore: 6 * time.Hour,

		Topics: map[string]*pubsub.TopicScoreParams{
		},
	}
}

func peerScoreThresholds() *pubsub.PeerScoreThresholds {
	// https://github.com/libp2p/specs/blob/master/pubsub/gossipsub/gossipsub-v1.1.md#overview-of-new-parameters
	return &pubsub.PeerScoreThresholds{
		GossipThreshold:             -1000,
		PublishThreshold:            -2000,
		GraylistThreshold:           -8000,
		AcceptPXThreshold:           1000,
		OpportunisticGraftThreshold: 5,
	}
}
