package fraud

import (
	"context"
	"time"

	"github.com/ipfs/go-datastore"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/celestiaorg/go-libp2p-messenger/serde"

	pb "github.com/celestiaorg/celestia-node/libs/fraud/pb"
)

// syncFraudProofs encompasses the behavior for fetching fraud proofs from other peers.
// syncFraudProofs subscribes to EvtPeerIdentificationCompleted to get newly connected peers
// to request fraud proofs from and request fraud proofs from them.
// After fraud proofs are received, they are published to all local subscriptions for verification
// order to be verified.
func (f *ProofService) syncFraudProofs(ctx context.Context, id protocol.ID) {
	log.Debug("start fetching fraud proofs")
	// subscribe to new peer connections that we can request fraud proofs from
	sub, err := f.host.EventBus().Subscribe(&event.EvtPeerIdentificationCompleted{})
	if err != nil {
		log.Error(err)
		return
	}
	defer sub.Close()
	f.topicsLk.RLock()
	// get proof types from subscribed pubsub topics
	proofTypes := make([]string, 0, len(f.topics))
	for proofType := range f.topics {
		proofTypes = append(proofTypes, string(proofType))
	}
	f.topicsLk.RUnlock()
	connStatus := event.EvtPeerIdentificationCompleted{}
	// peerCache is used to store discovered peers to avoid sending multiple requests to the same peer
	peerCache := make(map[peer.ID]struct{})
	// request proofs from `fraudRequests` many peers
	for i := 0; i < fraudRequests; i++ {
		select {
		case <-ctx.Done():
			return
		case e := <-sub.Out():
			connStatus = e.(event.EvtPeerIdentificationCompleted)
		}

		// ignore already requested peers or ourselves as a peer
		if _, ok := peerCache[connStatus.Peer]; ok || connStatus.Peer == f.host.ID() {
			i--
			continue
		}

		peerCache[connStatus.Peer] = struct{}{}
		// valid peer found, so go send proof requests
		go func(pid peer.ID) {
			ctx, span := tracer.Start(ctx, "sync_proofs")
			defer span.End()

			span.SetAttributes(
				attribute.String("peer_id", pid.String()),
				attribute.StringSlice("proof_types", proofTypes),
			)
			log.Debugw("requesting proofs from peer", "pid", pid)
			respProofs, err := f.requestProofs(ctx, id, pid, proofTypes)
			if err != nil {
				log.Errorw("error while requesting fraud proofs", "err", err, "peer", pid)
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				return
			}
			if len(respProofs) == 0 {
				log.Debugw("peer did not return any proofs", "pid", pid)
				span.SetStatus(codes.Ok, "")
				return
			}
			log.Debugw("got fraud proofs from peer", "pid", pid)
			for _, data := range respProofs {
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
						span.RecordError(err)
					}
				}
			}
			span.SetStatus(codes.Ok, "")
		}(connStatus.Peer)
	}
}

// handleFraudMessageRequest handles an incoming FraudMessageRequest.
func (f *ProofService) handleFraudMessageRequest(stream network.Stream) {
	req := &pb.FraudMessageRequest{}
	if err := stream.SetReadDeadline(time.Now().Add(readDeadline)); err != nil {
		log.Warn(err)
	}
	_, err := serde.Read(stream, req)
	if err != nil {
		stream.Reset() //nolint:errcheck
		log.Errorw("handling fraud message request failed", "err", err)
		return
	}
	if err = stream.CloseRead(); err != nil {
		log.Warn(err)
	}

	resp := &pb.FraudMessageResponse{}
	resp.Proofs = make([]*pb.ProofResponse, 0, len(req.RequestedProofType))
	// retrieve fraud proofs as provided by the FraudMessageRequest proofTypes.
	for _, p := range req.RequestedProofType {
		proofs, err := f.Get(f.ctx, ProofType(p))
		if err != nil {
			if err != datastore.ErrNotFound {
				log.Error(err)
			}
			continue
		}
		pbProofs := &pb.ProofResponse{Type: p, Value: make([][]byte, 0, len(proofs))}
		for _, proof := range proofs {
			bin, err := proof.MarshalBinary()
			if err != nil {
				log.Error(err)
				continue
			}
			pbProofs.Value = append(pbProofs.Value, bin)
		}
		resp.Proofs = append(resp.Proofs, pbProofs)
	}

	if err = stream.SetWriteDeadline(time.Now().Add(writeDeadline)); err != nil {
		log.Warn(err)
	}
	_, err = serde.Write(stream, resp)
	if err != nil {
		stream.Reset() //nolint:errcheck
		log.Errorw("error while writing a response", "err", err)
		return
	}
	if err = stream.Close(); err != nil {
		log.Errorw("error while closing a writer in stream", "err", err)
	}
}
