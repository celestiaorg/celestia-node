package fraudserv

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/celestiaorg/celestia-node/libs/fraud"
)

var (
	log    = logging.Logger("fraudserv")
	tracer = otel.Tracer("fraudserv")
)

// fraudRequests is the amount of external requests that will be tried to get fraud proofs from
// other peers.
const fraudRequests = 5

// ProofService is responsible for validating and propagating Fraud Proofs.
// It implements the Service interface.
type ProofService struct {
	networkID string

	ctx    context.Context
	cancel context.CancelFunc

	topicsLk sync.RWMutex
	topics   map[fraud.ProofType]*pubsub.Topic

	storesLk sync.RWMutex
	stores   map[fraud.ProofType]datastore.Datastore

	pubsub *pubsub.PubSub
	host   host.Host
	getter fraud.HeaderFetcher
	ds     datastore.Datastore

	syncerEnabled bool
}

func NewProofService(
	p *pubsub.PubSub,
	host host.Host,
	getter fraud.HeaderFetcher,
	ds datastore.Datastore,
	syncerEnabled bool,
	networkID string,
) *ProofService {
	return &ProofService{
		pubsub:        p,
		host:          host,
		getter:        getter,
		topics:        make(map[fraud.ProofType]*pubsub.Topic),
		stores:        make(map[fraud.ProofType]datastore.Datastore),
		ds:            ds,
		networkID:     networkID,
		syncerEnabled: syncerEnabled,
	}
}

// registerProofTopics registers proofTypes as pubsub topics to be joined.
func (f *ProofService) registerProofTopics(proofTypes ...fraud.ProofType) error {
	for _, proofType := range proofTypes {
		t, err := join(f.pubsub, proofType, f.networkID, f.processIncoming)
		if err != nil {
			return err
		}
		f.topicsLk.Lock()
		f.topics[proofType] = t
		f.topicsLk.Unlock()
	}
	return nil
}

// Start joins fraud proofs topics, sets the stream handler for fraudProtocolID and starts syncing
// if syncer is enabled.
func (f *ProofService) Start(context.Context) error {
	f.ctx, f.cancel = context.WithCancel(context.Background())
	if err := f.registerProofTopics(fraud.Registered()...); err != nil {
		return err
	}
	id := protocolID(f.networkID)
	log.Infow("starting fraud proof service", "protocol ID", id)

	f.host.SetStreamHandler(id, f.handleFraudMessageRequest)
	if f.syncerEnabled {
		go f.syncFraudProofs(f.ctx, id)
	}
	return nil
}

// Stop removes the stream handler and cancels the underlying ProofService
func (f *ProofService) Stop(context.Context) (err error) {
	f.host.RemoveStreamHandler(protocolID(f.networkID))
	f.cancel()
	return
}

func (f *ProofService) Subscribe(proofType fraud.ProofType) (_ fraud.Subscription, err error) {
	f.topicsLk.Lock()
	defer f.topicsLk.Unlock()
	t, ok := f.topics[proofType]
	if !ok {
		return nil, fmt.Errorf("topic for %s does not exist", proofType)
	}
	subs, err := t.Subscribe()
	if err != nil {
		return nil, err
	}
	return &subscription{subs}, nil
}

func (f *ProofService) Broadcast(ctx context.Context, p fraud.Proof) error {
	bin, err := p.MarshalBinary()
	if err != nil {
		return err
	}
	f.topicsLk.RLock()
	t, ok := f.topics[p.Type()]
	f.topicsLk.RUnlock()
	if !ok {
		return fmt.Errorf("fraud: unmarshaler for %s proof is not registered", p.Type())
	}
	return t.Publish(ctx, bin)
}

// processIncoming encompasses the logic for validating fraud proofs.
func (f *ProofService) processIncoming(
	ctx context.Context,
	proofType fraud.ProofType,
	from peer.ID,
	msg *pubsub.Message,
) pubsub.ValidationResult {
	ctx, span := tracer.Start(ctx, "process_proof", trace.WithAttributes(
		attribute.String("proof_type", string(proofType)),
	))
	defer span.End()

	// unmarshal message to the Proof.
	// Peer will be added to black list if unmarshalling fails.
	proof, err := fraud.Unmarshal(proofType, msg.Data)
	if err != nil {
		log.Errorw("unmarshalling failed", "err", err)
		if !errors.Is(err, &fraud.ErrNoUnmarshaler{}) {
			f.pubsub.BlacklistPeer(from)
		}
		span.RecordError(err)
		return pubsub.ValidationReject
	}
	// check the fraud proof locally and ignore if it has been already stored locally.
	if f.verifyLocal(ctx, proofType, hex.EncodeToString(proof.HeaderHash()), msg.Data) {
		span.AddEvent("received_known_fraud_proof", trace.WithAttributes(
			attribute.String("proof_type", string(proof.Type())),
			attribute.Int("block_height", int(proof.Height())),
			attribute.String("block_hash", hex.EncodeToString(proof.HeaderHash())),
			attribute.String("from_peer", from.String()),
		))
		return pubsub.ValidationIgnore
	}

	msg.ValidatorData = proof

	// fetch extended header in order to verify the fraud proof.
	extHeader, err := f.getter(ctx, proof.Height())
	if err != nil {
		log.Errorw("failed to fetch header to verify a fraud proof",
			"err", err, "proofType", proof.Type(), "height", proof.Height())
		return pubsub.ValidationIgnore
	}
	// validate the fraud proof.
	// Peer will be added to black list if the validation fails.
	err = proof.Validate(extHeader)
	if err != nil {
		log.Errorw("proof validation err: ",
			"err", err, "proofType", proof.Type(), "height", proof.Height())
		f.pubsub.BlacklistPeer(from)
		span.RecordError(err)
		return pubsub.ValidationReject
	}

	span.AddEvent("received_valid_proof", trace.WithAttributes(
		attribute.String("proof_type", string(proof.Type())),
		attribute.Int("block_height", int(proof.Height())),
		attribute.String("block_hash", hex.EncodeToString(proof.HeaderHash())),
		attribute.String("from_peer", from.String()),
	))

	// add the fraud proof to storage.
	err = f.put(ctx, proof.Type(), hex.EncodeToString(proof.HeaderHash()), msg.Data)
	if err != nil {
		log.Errorw("failed to store fraud proof", "err", err)
		span.RecordError(err)
	}

	span.SetStatus(codes.Ok, "")
	return pubsub.ValidationAccept
}

func (f *ProofService) Get(ctx context.Context, proofType fraud.ProofType) ([]fraud.Proof, error) {
	f.storesLk.Lock()
	store, ok := f.stores[proofType]
	if !ok {
		store = initStore(proofType, f.ds)
		f.stores[proofType] = store
	}
	f.storesLk.Unlock()

	return getAll(ctx, store, proofType)
}

// put adds a fraud proof to the local storage.
func (f *ProofService) put(ctx context.Context, proofType fraud.ProofType, hash string, data []byte) error {
	f.storesLk.Lock()
	store, ok := f.stores[proofType]
	if !ok {
		store = initStore(proofType, f.ds)
		f.stores[proofType] = store
	}
	f.storesLk.Unlock()
	return put(ctx, store, hash, data)
}

// verifyLocal checks if a fraud proof has been stored locally.
func (f *ProofService) verifyLocal(ctx context.Context, proofType fraud.ProofType, hash string, data []byte) bool {
	f.storesLk.RLock()
	storage, ok := f.stores[proofType]
	f.storesLk.RUnlock()
	if !ok {
		return false
	}

	proof, err := getByHash(ctx, storage, hash)
	if err != nil {
		if !errors.Is(err, datastore.ErrNotFound) {
			log.Error(err)
		}
		return false
	}

	return bytes.Equal(proof, data)
}
