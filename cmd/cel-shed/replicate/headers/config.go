package headers

import (
	"fmt"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

type Config struct {
	Source         string
	DataDir        string
	Network        modp2p.Network
	FromHeight     uint64
	ToHeight       uint64
	Concurrency    int
	RequestTimeout time.Duration
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Source) == "" {
		return fmt.Errorf("headers: source multiaddr is required")
	}
	info, err := peer.AddrInfoFromString(c.Source)
	if err != nil {
		return fmt.Errorf("headers: parse source multiaddr: %w", err)
	}
	if info.ID == "" {
		return fmt.Errorf("headers: source must include /p2p/<peer-id>")
	}
	if len(info.Addrs) == 0 {
		return fmt.Errorf("headers: source must include at least one dialable multiaddr before /p2p/<peer-id>")
	}
	if c.DataDir == "" {
		return fmt.Errorf("headers: data dir is required")
	}
	if c.Concurrency < 1 || c.Concurrency > 32 {
		return fmt.Errorf("headers: concurrency must be between 1 and 32, got %d", c.Concurrency)
	}
	if c.RequestTimeout < time.Second {
		return fmt.Errorf("headers: request-timeout must be >= 1s, got %s", c.RequestTimeout)
	}
	if c.ToHeight != 0 && c.FromHeight != 0 && c.FromHeight > c.ToHeight {
		return fmt.Errorf("headers: from-height (%d) must be <= to-height (%d)", c.FromHeight, c.ToHeight)
	}
	return nil
}
