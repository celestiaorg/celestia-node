package share

import (
	"context"
	"fmt"
	"sync"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
)

// CDASubnetManager manages pubsub topics for CDA:
// - Cell topic: cda/cell/<row>/<col> for raw share distribution
// - Column topic: cda/col/<col> for coded fragment gossip
type CDASubnetManager struct {
	pubsub      *pubsub.PubSub
	gridManager *RDAGridManager
	myPeerID    peer.ID
	myPos       GridPosition

	cellTopic string
	colTopic  string

	cellSub *pubsub.Subscription
	colSub  *pubsub.Subscription

	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

func NewCDASubnetManager(ps *pubsub.PubSub, gridManager *RDAGridManager, myPeerID peer.ID) *CDASubnetManager {
	ctx, cancel := context.WithCancel(context.Background())

	pos := GetCoords(myPeerID, gridManager.GetGridDimensions())
	cellTopic := fmt.Sprintf("cda/cell/%d/%d", pos.Row, pos.Col)
	colTopic := fmt.Sprintf("cda/col/%d", pos.Col)

	return &CDASubnetManager{
		pubsub:      ps,
		gridManager: gridManager,
		myPeerID:    myPeerID,
		myPos:       pos,
		cellTopic:   cellTopic,
		colTopic:    colTopic,
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (s *CDASubnetManager) Start(ctx context.Context) error {
	cellT, err := s.pubsub.Join(s.cellTopic)
	if err != nil {
		return fmt.Errorf("cda: join cell topic: %w", err)
	}
	cellSub, err := cellT.Subscribe()
	if err != nil {
		cellT.Close()
		return fmt.Errorf("cda: subscribe cell topic: %w", err)
	}

	colT, err := s.pubsub.Join(s.colTopic)
	if err != nil {
		cellSub.Cancel()
		cellT.Close()
		return fmt.Errorf("cda: join col topic: %w", err)
	}
	colSub, err := colT.Subscribe()
	if err != nil {
		cellSub.Cancel()
		cellT.Close()
		colT.Close()
		return fmt.Errorf("cda: subscribe col topic: %w", err)
	}

	s.mu.Lock()
	s.cellSub = cellSub
	s.colSub = colSub
	s.mu.Unlock()

	log.Infof("Subscribed to CDA topics - Cell: %s, Col: %s", s.cellTopic, s.colTopic)
	return nil
}

func (s *CDASubnetManager) Stop(ctx context.Context) error {
	s.cancel()
	s.mu.Lock()
	if s.cellSub != nil {
		s.cellSub.Cancel()
	}
	if s.colSub != nil {
		s.colSub.Cancel()
	}
	s.mu.Unlock()
	return nil
}

func (s *CDASubnetManager) PublishToCell(ctx context.Context, data []byte) error {
	t, err := s.pubsub.Join(s.cellTopic)
	if err != nil {
		return fmt.Errorf("cda: join cell topic: %w", err)
	}
	defer t.Close()
	if err := t.Publish(ctx, data); err != nil {
		return fmt.Errorf("cda: publish cell topic: %w", err)
	}
	return nil
}

func (s *CDASubnetManager) PublishToCol(ctx context.Context, data []byte) error {
	t, err := s.pubsub.Join(s.colTopic)
	if err != nil {
		return fmt.Errorf("cda: join col topic: %w", err)
	}
	defer t.Close()
	if err := t.Publish(ctx, data); err != nil {
		return fmt.Errorf("cda: publish col topic: %w", err)
	}
	return nil
}

func (s *CDASubnetManager) ReceiveFromCell(ctx context.Context) <-chan []byte {
	out := make(chan []byte, 100)
	go func() {
		defer close(out)
		s.mu.RLock()
		sub := s.cellSub
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

func (s *CDASubnetManager) ReceiveFromCol(ctx context.Context) <-chan []byte {
	out := make(chan []byte, 100)
	go func() {
		defer close(out)
		s.mu.RLock()
		sub := s.colSub
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

func (s *CDASubnetManager) GetCellTopic() string { return s.cellTopic }
func (s *CDASubnetManager) GetColTopic() string  { return s.colTopic }

