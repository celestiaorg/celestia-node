package replicate

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	modp2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
)

const DefaultBatchSize = 10_000

type Config struct {
	RemoteHost     string
	RemoteBlocks   string
	Source         string
	DataDir        string
	Network        modp2p.Network
	FromHeight     uint64
	ToHeight       uint64
	BatchSize      uint64
	Concurrency    int
	RequestTimeout time.Duration
	LogLevel       string
	Verify         bool
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.RemoteHost) == "" {
		return fmt.Errorf("remote-host is required")
	}
	if strings.TrimSpace(c.RemoteBlocks) == "" {
		return fmt.Errorf("remote-blocks is required")
	}
	if !filepath.IsAbs(c.RemoteBlocks) {
		return fmt.Errorf("remote-blocks must be absolute, got %q", c.RemoteBlocks)
	}
	if strings.TrimSpace(c.Source) == "" {
		return fmt.Errorf("source multiaddr is required")
	}
	if strings.TrimSpace(c.DataDir) == "" {
		return fmt.Errorf("data-dir is required")
	}
	if c.BatchSize == 0 {
		return fmt.Errorf("batch-size must be > 0")
	}
	if c.Concurrency < 1 || c.Concurrency > 32 {
		return fmt.Errorf("concurrency must be between 1 and 32, got %d", c.Concurrency)
	}
	if c.RequestTimeout < time.Second {
		return fmt.Errorf("request-timeout must be >= 1s, got %s", c.RequestTimeout)
	}
	if c.FromHeight != 0 && c.ToHeight != 0 && c.FromHeight > c.ToHeight {
		return fmt.Errorf("from-height (%d) must be <= to-height (%d)", c.FromHeight, c.ToHeight)
	}
	return nil
}

func (c Config) workdir() string {
	return filepath.Join(c.DataDir, ".cel-shed-replicate")
}
