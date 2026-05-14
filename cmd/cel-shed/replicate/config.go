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
	Recover        bool
	RecoverMax     uint64
	HeadersOnly    bool
}

func (c Config) Validate() error {
	if c.Recover && c.HeadersOnly {
		return fmt.Errorf("recover and headers-only cannot both be set")
	}
	if !c.HeadersOnly && strings.TrimSpace(c.RemoteHost) == "" {
		return fmt.Errorf("remote-host is required")
	}
	if !c.HeadersOnly && strings.TrimSpace(c.RemoteBlocks) == "" {
		return fmt.Errorf("remote-blocks is required")
	}
	if !c.HeadersOnly && !filepath.IsAbs(c.RemoteBlocks) {
		return fmt.Errorf("remote-blocks must be absolute, got %q", c.RemoteBlocks)
	}
	if !c.Recover && strings.TrimSpace(c.Source) == "" {
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
	if c.Recover {
		if c.RecoverMax == 0 {
			return fmt.Errorf("recover-max-height is required in recover mode")
		}
		if c.FromHeight != 0 && c.FromHeight > c.RecoverMax {
			return fmt.Errorf("from-height (%d) must be <= recover-max-height (%d)", c.FromHeight, c.RecoverMax)
		}
	} else if c.FromHeight != 0 && c.ToHeight != 0 && c.FromHeight > c.ToHeight {
		return fmt.Errorf("from-height (%d) must be <= to-height (%d)", c.FromHeight, c.ToHeight)
	}
	return nil
}

func (c Config) workdir() string {
	return filepath.Join(c.DataDir, ".cel-shed-replicate")
}
