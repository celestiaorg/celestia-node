package share

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

// SubnetAnnouncement is a message broadcast within a subnet to announce
// a node's presence and listen for other members.
type SubnetAnnouncement struct {
	NodeID    string   // Peer ID of the announcing node
	PeerAddrs []string // Multiaddrs where this node can be reached
	Timestamp int64    // Unix nano timestamp
	Role      string   // "normal", "bootstrap", etc.
}

// SubnetMember represents a discovered member in a subnet
type SubnetMember struct {
	PeerID    peer.ID
	PeerAddrs []peer.AddrInfo
	LastSeen  time.Time
}

// SubnetAnnouncer handles announcement and discovery within a single subnet
type SubnetAnnouncer struct {
	subnet  string
	host    host.Host
	pubsub  *pubsub.PubSub
	myID    peer.ID
	myAddrs []string

	// Subscription to announcement topic
	sub *pubsub.Subscription

	// Collected members from announcements
	mu      sync.RWMutex
	members map[string]SubnetMember

	// Delay configuration for pull phase
	delayBeforePull time.Duration
	pullTimeout     time.Duration

	// Channels for notifications
	memberCh chan SubnetMember

	// Lifecycle
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewSubnetAnnouncer creates a new subnet announcement handler
func NewSubnetAnnouncer(
	ctx context.Context,
	host host.Host,
	pubsub *pubsub.PubSub,
	subnetName string,
	delayBeforePull time.Duration,
) (*SubnetAnnouncer, error) {
	// Subscribe to announcements on this subnet
	topic := fmt.Sprintf("rda/subnet/%s/announce", subnetName)
	sub, err := pubsub.Subscribe(topic)
	if err != nil {
		return nil, fmt.Errorf("subscribe to subnet %s: %w", subnetName, err)
	}

	sa := &SubnetAnnouncer{
		subnet:          subnetName,
		host:            host,
		pubsub:          pubsub,
		myID:            host.ID(),
		myAddrs:         multiaddrsToStrings(host.Addrs()),
		sub:             sub,
		members:         make(map[string]SubnetMember),
		delayBeforePull: delayBeforePull,
		pullTimeout:     5 * time.Second,
		memberCh:        make(chan SubnetMember, 100),
	}

	// Start listening for announcements
	ctx, cancel := context.WithCancel(context.Background())
	sa.cancel = cancel

	sa.wg.Add(1)
	go sa.listenAnnouncements(ctx)

	return sa, nil
}

// listenAnnouncements continuously listens for announcements on the subnet
func (sa *SubnetAnnouncer) listenAnnouncements(ctx context.Context) {
	defer sa.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msg, err := sa.sub.Next(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			continue
		}

		// Deserialize announcement
		var announcement SubnetAnnouncement
		if err := deserializeAnnouncement(msg.Data, &announcement); err != nil {
			continue
		}

		// Skip own announcements
		if announcement.NodeID == sa.myID.String() {
			continue
		}

		// Convert multiaddr strings to peer.AddrInfo
		peerID, err := peer.Decode(announcement.NodeID)
		if err != nil {
			continue
		}

		addrs, err := stringsToPeerAddrs(announcement.PeerAddrs, peerID)
		if err != nil {
			continue
		}

		// Record member
		member := SubnetMember{
			PeerID:    peerID,
			PeerAddrs: addrs,
			LastSeen:  time.Now(),
		}

		sa.mu.Lock()
		sa.members[peerID.String()] = member
		sa.mu.Unlock()

		// Notify subscribers
		select {
		case sa.memberCh <- member:
		default:
			// Channel full, skip
		}
	}
}

// AnnounceJoin broadcasts this node's presence to the subnet
func (sa *SubnetAnnouncer) AnnounceJoin(ctx context.Context) error {
	announcement := SubnetAnnouncement{
		NodeID:    sa.myID.String(),
		PeerAddrs: sa.myAddrs,
		Timestamp: time.Now().UnixNano(),
		Role:      "normal",
	}

	data, err := serializeAnnouncement(&announcement)
	if err != nil {
		return err
	}

	topic := fmt.Sprintf("rda/subnet/%s/announce", sa.subnet)
	return sa.pubsub.Publish(topic, data)
}

// GetMembersAfterDelay waits for delay period, then collects all discovered members.
// This implements the "delayed pull" mechanism from the RDA paper.
func (sa *SubnetAnnouncer) GetMembersAfterDelay(ctx context.Context) []SubnetMember {
	// Wait for delay period to allow announcements to propagate
	select {
	case <-time.After(sa.delayBeforePull):
	case <-ctx.Done():
		return nil
	}

	sa.mu.RLock()
	defer sa.mu.RUnlock()

	members := make([]SubnetMember, 0, len(sa.members))
	for _, m := range sa.members {
		members = append(members, m)
	}
	return members
}

// MemberUpdates returns a channel that notifies when new members are discovered
func (sa *SubnetAnnouncer) MemberUpdates() <-chan SubnetMember {
	return sa.memberCh
}

// GetMembers returns currently known members
func (sa *SubnetAnnouncer) GetMembers() []SubnetMember {
	sa.mu.RLock()
	defer sa.mu.RUnlock()

	members := make([]SubnetMember, 0, len(sa.members))
	for _, m := range sa.members {
		members = append(members, m)
	}
	return members
}

// Stop shuts down the announcer
func (sa *SubnetAnnouncer) Stop(ctx context.Context) error {
	if sa.cancel != nil {
		sa.cancel()
	}
	sa.sub.Cancel()
	sa.wg.Wait()
	close(sa.memberCh)
	return nil
}

// Helper functions

func multiaddrsToStrings(addrs []ma.Multiaddr) []string {
	result := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		result = append(result, addr.String())
	}
	return result
}

func stringsToPeerAddrs(addrs []string, peerID peer.ID) ([]peer.AddrInfo, error) {
	result := make([]peer.AddrInfo, 0, len(addrs))
	for _, addrStr := range addrs {
		// Assume addresses are in format: /ip4/X.X.X.X/tcp/7777
		// Append p2p protocol component
		fullAddr := fmt.Sprintf("%s/p2p/%s", addrStr, peerID.String())
		maddr, err := ma.NewMultiaddr(fullAddr)
		if err != nil {
			continue
		}
		info, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			continue
		}
		result = append(result, *info)
	}
	return result, nil
}

func serializeAnnouncement(ann *SubnetAnnouncement) ([]byte, error) {
	return json.Marshal(ann)
}

func deserializeAnnouncement(data []byte, ann *SubnetAnnouncement) error {
	return json.Unmarshal(data, ann)
}

// RDASubnetDiscoveryManager manages multiple subnet announcers
type RDASubnetDiscoveryManager struct {
	host            host.Host
	pubsub          *pubsub.PubSub
	delayBeforePull time.Duration

	mu         sync.RWMutex
	announcers map[string]*SubnetAnnouncer
}

// NewRDASubnetDiscoveryManager creates a discovery manager
func NewRDASubnetDiscoveryManager(
	host host.Host,
	pubsub *pubsub.PubSub,
	delayBeforePull time.Duration,
) *RDASubnetDiscoveryManager {
	return &RDASubnetDiscoveryManager{
		host:            host,
		pubsub:          pubsub,
		delayBeforePull: delayBeforePull,
		announcers:      make(map[string]*SubnetAnnouncer),
	}
}

// GetOrCreateAnnouncer gets or creates an announcer for a subnet
func (m *RDASubnetDiscoveryManager) GetOrCreateAnnouncer(
	ctx context.Context,
	subnet string,
) (*SubnetAnnouncer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if announcer, exists := m.announcers[subnet]; exists {
		return announcer, nil
	}

	announcer, err := NewSubnetAnnouncer(ctx, m.host, m.pubsub, subnet, m.delayBeforePull)
	if err != nil {
		return nil, err
	}

	m.announcers[subnet] = announcer
	return announcer, nil
}

// Stop stops all announcers
func (m *RDASubnetDiscoveryManager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, announcer := range m.announcers {
		_ = announcer.Stop(ctx)
	}
	m.announcers = make(map[string]*SubnetAnnouncer)
	return nil
}
