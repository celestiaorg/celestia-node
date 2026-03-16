package share

import (
	"errors"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/celestiaorg/celestia-node/share/crypto"
)

// CDAServiceConfig holds CDA-specific parameters.
type CDAServiceConfig struct {
	// K is the target number of fragments required for reconstruction.
	K uint16
	// Buffer is the additional per-share storage headroom beyond K.
	Buffer uint16
}

// CDAService provides storage policy and core primitives for CDA fragments.
// Networking (pubsub / streams) will be wired in via nodebuilder in later steps.
type CDAService struct {
	cfg CDAServiceConfig

	mu sync.RWMutex
	// storedFragments enforces:
	// - Peer-ID binding: at most one fragment per peer per share coordinate
	// - Hard cap: at most K+Buffer fragments per share coordinate
	storedFragments map[FragmentKey]map[peer.ID]Fragment
}

var ErrCDAInvalidFragment = errors.New("cda: invalid fragment")

// FragmentKey groups fragments by (height,row,col) for a specific share coordinate.
type FragmentKey struct {
	Height uint64
	Row    uint16
	Col    uint16
}

func NewCDAService(cfg CDAServiceConfig) *CDAService {
	if cfg.K == 0 {
		cfg.K = 16
	}
	return &CDAService{
		cfg:             cfg,
		storedFragments: make(map[FragmentKey]map[peer.ID]Fragment),
	}
}

// StoreFragment attempts to store a fragment, enforcing anti-spam policy.
// It returns true if stored, false otherwise.
func (s *CDAService) StoreFragment(f Fragment) bool {
	key := FragmentKey{
		Height: f.ID.Height,
		Row:    f.ID.Row,
		Col:    f.ID.Col,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	byPeer, ok := s.storedFragments[key]
	if !ok {
		byPeer = make(map[peer.ID]Fragment)
		s.storedFragments[key] = byPeer
	}

	// Peer-ID binding
	if _, exists := byPeer[f.ID.NodeID]; exists {
		return false
	}

	// Hard cap
	max := int(s.cfg.K + s.cfg.Buffer)
	if max > 0 && len(byPeer) >= max {
		return false
	}

	byPeer[f.ID.NodeID] = f
	return true
}

// ValidateAndStoreFragment verifies a fragment against column commitments and stores it if:
// - cryptographically valid (KZG verification succeeds), and
// - it satisfies anti-spam storage policy (peer-id binding + hard cap).
//
// If the hard cap is reached or the peer already stored a fragment for the same key,
// the fragment is not stored and callers should not re-propagate it.
func (s *CDAService) ValidateAndStoreFragment(
	verifier crypto.HomomorphicKZG,
	colComs []crypto.ColumnCommitment,
	f Fragment,
) (bool, error) {
	if verifier == nil {
		return false, errors.New("cda: nil kzg verifier")
	}

	if err := verifier.VerifyFragment(f.Data, f.CodingVec, colComs, f.Proof); err != nil {
		return false, ErrCDAInvalidFragment
	}

	if ok := s.StoreFragment(f); !ok {
		return false, nil
	}

	return true, nil
}

// GetFragments returns currently stored fragments for the given share coordinate.
func (s *CDAService) GetFragments(height uint64, row, col uint16) []Fragment {
	key := FragmentKey{Height: height, Row: row, Col: col}

	s.mu.RLock()
	defer s.mu.RUnlock()

	byPeer := s.storedFragments[key]
	out := make([]Fragment, 0, len(byPeer))
	for _, f := range byPeer {
		out = append(out, f)
	}
	return out
}

