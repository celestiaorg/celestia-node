package headers

import (
	"fmt"

	"github.com/celestiaorg/celestia-node/header"
)

// validateFetchedChunk checks shape only: the returned slice matches the
// requested [start, end] window in length and per-index heights, and starts
// where the writer expects.
func validateFetchedChunk(c chunk, hdrs []*header.ExtendedHeader, wantStart uint64) error {
	if len(hdrs) == 0 {
		return fmt.Errorf("0 headers for range %d..%d", c.start, c.end)
	}
	if c.start != wantStart {
		return fmt.Errorf("unexpected chunk start: got %d want %d", c.start, wantStart)
	}

	wantLen := int(c.end - c.start + 1)
	if len(hdrs) != wantLen {
		return fmt.Errorf("wrong number of headers for range %d..%d: got %d want %d",
			c.start, c.end, len(hdrs), wantLen)
	}

	for i, h := range hdrs {
		wantHeight := c.start + uint64(i)
		if h.Height() != wantHeight {
			return fmt.Errorf("unexpected header height at offset %d: got %d want %d",
				i, h.Height(), wantHeight)
		}
	}
	return nil
}

// verifyChunkAgainstLocalTip runs the cryptographic adjacency check between
// the locally trusted tip and the first header of the chunk, then walks the
// chunk pairwise. go-header does not expose a batched VerifyRange, so this
// substitutes the equivalent N-1 calls to (*ExtendedHeader).Verify.
func verifyChunkAgainstLocalTip(
	last *header.ExtendedHeader,
	hdrs []*header.ExtendedHeader,
) error {
	if len(hdrs) == 0 {
		return fmt.Errorf("empty header chunk")
	}
	if last == nil {
		return nil
	}
	if hdrs[0].Height() != last.Height()+1 {
		return fmt.Errorf("non-adjacent chunk: first height %d after local tip %d",
			hdrs[0].Height(), last.Height())
	}

	if err := last.Verify(hdrs[0]); err != nil {
		return fmt.Errorf("verify first header %d against local tip %d: %w",
			hdrs[0].Height(), last.Height(), err)
	}
	for i := 1; i < len(hdrs); i++ {
		if err := hdrs[i-1].Verify(hdrs[i]); err != nil {
			return fmt.Errorf("verify header %d against %d: %w",
				hdrs[i].Height(), hdrs[i-1].Height(), err)
		}
	}
	return nil
}

func validateChunkChainID(hdrs []*header.ExtendedHeader, chainID string) error {
	for _, h := range hdrs {
		if h.ChainID() != chainID {
			return fmt.Errorf("header %d chain-id mismatch: got %s want %s",
				h.Height(), h.ChainID(), chainID)
		}
	}
	return nil
}
