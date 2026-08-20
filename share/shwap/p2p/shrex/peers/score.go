package peers

import (
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	peerStatsCacheSize = 1024
	defaultPeerScore   = 10 << 20
	peerScoreAlpha     = 0.25
	peerScoreDecay     = 0.8
)

// TransferStats describes a successful data transfer reported through DoneFunc.
type TransferStats struct {
	Bytes    int64
	Duration time.Duration
}

type peerStat struct {
	score    float64
	measured bool
}

// peerStats keeps an EWMA of observed throughput (bytes/s) per peer. It is shared by all pools
// of a Manager, so a peer keeps its score across data hashes.
type peerStats struct {
	m      sync.Mutex
	scores *lru.Cache[peer.ID, peerStat]
}

func newPeerStats() (*peerStats, error) {
	// Scores of evicted peers are measured again on their next successful request. Bounding the
	// cache only costs accuracy for peers we have not talked to in a long time.
	scores, err := lru.New[peer.ID, peerStat](peerStatsCacheSize)
	if err != nil {
		return nil, err
	}

	return &peerStats{
		scores: scores,
	}, nil
}

// score returns the peer's throughput in bytes/s and whether it has been measured.
func (s *peerStats) score(peerID peer.ID) (float64, bool) {
	s.m.Lock()
	defer s.m.Unlock()

	stat, ok := s.scores.Get(peerID)
	return stat.score, ok && stat.measured
}

// selectPeer returns the better of two peers. A new peer gets one priority selection, then uses
// defaultPeerScore until the request reports its result.
func (s *peerStats) selectPeer(first, second peer.ID) peer.ID {
	s.m.Lock()
	defer s.m.Unlock()

	firstStat, firstKnown := s.scores.Get(first)
	secondStat, secondKnown := s.scores.Get(second)

	selected := first
	switch {
	case firstKnown && !secondKnown:
		selected = second
	case firstKnown && secondKnown && secondStat.score > firstStat.score:
		selected = second
	}

	selectedKnown := firstKnown
	if selected == second {
		selectedKnown = secondKnown
	}
	if !selectedKnown {
		s.scores.Add(selected, peerStat{score: defaultPeerScore})
	}
	return selected
}

// updateStats folds a successful transfer into the peer's throughput score.
func (s *peerStats) updateStats(peerID peer.ID, stats TransferStats) {
	if stats.Bytes <= 0 || stats.Duration <= 0 {
		return
	}
	rate := float64(stats.Bytes) / stats.Duration.Seconds()

	s.m.Lock()
	defer s.m.Unlock()

	prev, ok := s.scores.Get(peerID)
	if !ok || !prev.measured {
		s.scores.Add(peerID, peerStat{score: rate, measured: true})
		return
	}
	prev.score += peerScoreAlpha * (rate - prev.score)
	s.scores.Add(peerID, prev)
}

// decreaseScore lowers a peer's score after a failed request. An unmeasured peer starts from
// defaultPeerScore so one failing peer cannot remain permanently preferred for exploration.
func (s *peerStats) decreaseScore(peerID peer.ID) {
	s.m.Lock()
	defer s.m.Unlock()

	stat, ok := s.scores.Get(peerID)
	if !ok {
		stat.score = defaultPeerScore
	}
	stat.score *= peerScoreDecay
	stat.measured = true
	s.scores.Add(peerID, stat)
}
