package share

import (
	"context"
	"fmt"
	"sync"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
)

// RDASubnetManager manages the registration and subscription to RDA grid subnets
type RDASubnetManager struct {
	pubsub      *pubsub.PubSub
	gridManager *RDAGridManager
	myPeerID    peer.ID
	myPosition  GridPosition

	// Subscriptions
	rowSubscription *pubsub.Subscription
	colSubscription *pubsub.Subscription

	// Topics
	rowTopic string
	colTopic string

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

// NewRDASubnetManager creates a new RDA subnet manager
func NewRDASubnetManager(
	ps *pubsub.PubSub,
	gridManager *RDAGridManager,
	myPeerID peer.ID,
) *RDASubnetManager {
	ctx, cancel := context.WithCancel(context.Background())

	position := GridPosition{}
	if pos, ok := gridManager.GetPeerPosition(myPeerID); ok {
		position = pos
	}

	rowID, colID := GetSubnetIDs(myPeerID, gridManager.GetGridDimensions())

	return &RDASubnetManager{
		pubsub:      ps,
		gridManager: gridManager,
		myPeerID:    myPeerID,
		myPosition:  position,
		rowTopic:    rowID,
		colTopic:    colID,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start registers and subscribes to the RDA grid subnets
func (s *RDASubnetManager) Start(ctx context.Context) error {
	// Subscribe to row topic
	rowTopic, err := s.pubsub.Join(s.rowTopic)
	if err != nil {
		return fmt.Errorf("failed to join row topic: %w", err)
	}

	rowSub, err := rowTopic.Subscribe()
	if err != nil {
		rowTopic.Close()
		return fmt.Errorf("failed to subscribe to row topic: %w", err)
	}

	// Subscribe to column topic
	colTopic, err := s.pubsub.Join(s.colTopic)
	if err != nil {
		rowTopic.Close()
		return fmt.Errorf("failed to join col topic: %w", err)
	}

	colSub, err := colTopic.Subscribe()
	if err != nil {
		rowTopic.Close()
		colTopic.Close()
		return fmt.Errorf("failed to subscribe to col topic: %w", err)
	}

	s.mu.Lock()
	s.rowSubscription = rowSub
	s.colSubscription = colSub
	s.mu.Unlock()

	log.Infof("Subscribed to RDA subnets - Row: %s, Col: %s", s.rowTopic, s.colTopic)

	return nil
}

// Stop stops the subnet manager and unsubscribes from topics
func (s *RDASubnetManager) Stop(ctx context.Context) error {
	s.cancel()

	s.mu.Lock()
	if s.rowSubscription != nil {
		s.rowSubscription.Cancel()
	}
	if s.colSubscription != nil {
		s.colSubscription.Cancel()
	}
	s.mu.Unlock()

	log.Infof("Unsubscribed from RDA subnets")
	return nil
}

// PublishToRow publishes a message to all peers in the same row
func (s *RDASubnetManager) PublishToRow(ctx context.Context, data []byte) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rowTopic, err := s.pubsub.Join(s.rowTopic)
	if err != nil {
		return fmt.Errorf("failed to join row topic: %w", err)
	}
	defer rowTopic.Close()

	if err := rowTopic.Publish(ctx, data); err != nil {
		return fmt.Errorf("failed to publish to row topic: %w", err)
	}

	return nil
}

// PublishToCol publishes a message to all peers in the same column
func (s *RDASubnetManager) PublishToCol(ctx context.Context, data []byte) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	colTopic, err := s.pubsub.Join(s.colTopic)
	if err != nil {
		return fmt.Errorf("failed to join col topic: %w", err)
	}
	defer colTopic.Close()

	if err := colTopic.Publish(ctx, data); err != nil {
		return fmt.Errorf("failed to publish to col topic: %w", err)
	}

	return nil
}

// PublishToSubnet publishes a message to both row and column peers
func (s *RDASubnetManager) PublishToSubnet(ctx context.Context, data []byte) error {
	if err := s.PublishToRow(ctx, data); err != nil {
		return err
	}
	return s.PublishToCol(ctx, data)
}

// ReceiveFromRow receives messages from row peers
// Returns a channel that will receive messages
func (s *RDASubnetManager) ReceiveFromRow(ctx context.Context) <-chan []byte {
	out := make(chan []byte, 100)

	go func() {
		defer close(out)
		s.mu.RLock()
		sub := s.rowSubscription
		s.mu.RUnlock()

		if sub == nil {
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-s.ctx.Done():
				return
			default:
				msg, err := sub.Next(ctx)
				if err != nil {
					log.Warnf("error receiving row message: %v", err)
					return
				}

				select {
				case out <- msg.Data:
				case <-ctx.Done():
					return
				case <-s.ctx.Done():
					return
				}
			}
		}
	}()

	return out
}

// ReceiveFromCol receives messages from column peers
// Returns a channel that will receive messages
func (s *RDASubnetManager) ReceiveFromCol(ctx context.Context) <-chan []byte {
	out := make(chan []byte, 100)

	go func() {
		defer close(out)
		s.mu.RLock()
		sub := s.colSubscription
		s.mu.RUnlock()

		if sub == nil {
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-s.ctx.Done():
				return
			default:
				msg, err := sub.Next(ctx)
				if err != nil {
					log.Warnf("error receiving col message: %v", err)
					return
				}

				select {
				case out <- msg.Data:
				case <-ctx.Done():
					return
				case <-s.ctx.Done():
					return
				}
			}
		}
	}()

	return out
}

// GetRowPeers returns the list of peers in the row topic
func (s *RDASubnetManager) GetRowPeers() []peer.ID {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rowTopic, err := s.pubsub.Join(s.rowTopic)
	if err != nil {
		log.Warnf("failed to join row topic: %v", err)
		return nil
	}
	defer rowTopic.Close()

	return rowTopic.ListPeers()
}

// GetColPeers returns the list of peers in the column topic
func (s *RDASubnetManager) GetColPeers() []peer.ID {
	s.mu.RLock()
	defer s.mu.RUnlock()

	colTopic, err := s.pubsub.Join(s.colTopic)
	if err != nil {
		log.Warnf("failed to join col topic: %v", err)
		return nil
	}
	defer colTopic.Close()

	return colTopic.ListPeers()
}

// GetSubnetPeers returns all peers in row and column topics
func (s *RDASubnetManager) GetSubnetPeers() []peer.ID {
	peerSet := make(map[peer.ID]bool)

	for _, p := range s.GetRowPeers() {
		peerSet[p] = true
	}

	for _, p := range s.GetColPeers() {
		peerSet[p] = true
	}

	peers := make([]peer.ID, 0, len(peerSet))
	for p := range peerSet {
		peers = append(peers, p)
	}

	return peers
}

// GetRowTopic returns the row topic name
func (s *RDASubnetManager) GetRowTopic() string {
	return s.rowTopic
}

// GetColTopic returns the column topic name
func (s *RDASubnetManager) GetColTopic() string {
	return s.colTopic
}

// GetMyPosition returns the grid position
func (s *RDASubnetManager) GetMyPosition() GridPosition {
	return s.myPosition
}
