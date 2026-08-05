package peers

import (
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Sample is an observed data transfer, reported by the caller of Peer through DoneFunc.
type Sample struct {
	Bytes    int64
	Duration time.Duration
}

// scoreboard keeps an EWMA of observed throughput (bytes/s) per peer. It is shared by all pools
// of a Manager, so a peer keeps its score across datahashes.
type scoreboard struct {
	m      sync.Mutex
	scores *lru.Cache[peer.ID, float64]

	// seed is the score of a peer that has not been measured yet.
	seed float64
	// alpha is the weight given to the newest sample.
	alpha float64
}

func newScoreboard() (*scoreboard, error) {
	// scores of evicted peers are simply re-seeded on next observation, so bounding the cache
	// only costs accuracy for peers we have not talked to in a long time.
	scores, err := lru.New[peer.ID, float64](1024)
	if err != nil {
		return nil, err
	}

	return &scoreboard{
		scores: scores,
		// optimistic seed: above the throughput a single peer serves in practice, so an
		// unmeasured peer wins against measured mediocre ones and gets a chance to prove itself
		seed: 10 << 20,
		// adapt over a handful of requests
		alpha: 0.25,
	}, nil
}

// score returns the peer's throughput in bytes/s.
func (s *scoreboard) score(peerID peer.ID) float64 {
	s.m.Lock()
	defer s.m.Unlock()

	if score, ok := s.scores.Get(peerID); ok {
		return score
	}
	return s.seed
}

// observe folds the sample into the peer's throughput score.
func (s *scoreboard) observe(peerID peer.ID, sample Sample) {
	if sample.Bytes <= 0 || sample.Duration <= 0 {
		return
	}
	rate := float64(sample.Bytes) / sample.Duration.Seconds()

	s.m.Lock()
	defer s.m.Unlock()

	prev, ok := s.scores.Get(peerID)
	if !ok {
		prev = s.seed
	}
	s.scores.Add(peerID, prev+s.alpha*(rate-prev))
}
