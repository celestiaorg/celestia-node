package fraud

import (
	"context"

	"github.com/libp2p/go-libp2p-core/event"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

func (f *ProofService) syncFraudProofs(ctx context.Context) {
	log.Debug("start fetching Fraud Proofs")
	sub, err := f.host.EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
	if err != nil {
		log.Error(err)
		return
	}
	defer sub.Close()
	f.topicsLk.RLock()
	proofTypes := make([]uint32, 0, len(f.topics))
	for proofType := range f.topics {
		proofTypes = append(proofTypes, uint32(proofType))
	}
	f.topicsLk.RUnlock()
	connStatus := event.EvtPeerIdentificationCompleted{}
	for i := 0; i < fraudRequests; i++ {
		select {
		case <-ctx.Done():
			return
		case e := <-sub.Out():
			connStatus = e.(event.EvtPeerIdentificationCompleted)
		}

		if connStatus.Peer == f.host.ID() {
			i--
			continue
		}
		go func(pid peer.ID) {
			log.Debug("requesting proofs from the peer ", pid)
			respProofs, err := requestProofs(ctx, f.host, pid, proofTypes)
			if err != nil {
				log.Errorw("error while requesting fraud proofs from the peer", "err", err, "peer", pid)
				return
			}
			if len(respProofs) == 0 {
				log.Debug("proofs were not found")
				return
			}
			log.Debug("got fraud proofs from the peer: ", connStatus.Peer)
			for _, data := range respProofs {
				if err != nil {
					log.Warn(err)
					continue
				}
				f.topicsLk.RLock()
				topic, ok := f.topics[ProofType(data.Type)]
				f.topicsLk.RUnlock()
				if !ok {
					log.Errorf("topic for %s does not exist", ProofType(data.Type))
					continue
				}
				for _, val := range data.Value {
					err = topic.Publish(
						ctx,
						val,
						// broadcast across all local subscriptions in order to verify fraud proof and to stop services
						pubsub.WithLocalPublication(true),
						// key can be nil because it will not be verified in this case
						pubsub.WithSecretKeyAndPeerId(nil, pid),
					)
					if err != nil {
						log.Error(err)
					}
				}
			}
		}(connStatus.Peer)
	}
}
