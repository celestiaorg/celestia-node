package fibre

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	appfibre "github.com/celestiaorg/celestia-app/v8/fibre"

	"github.com/celestiaorg/celestia-node/state"
)

// TxConfig aliases TxOptions from state package allowing users
// to specify options for SubmitPFB transaction.
type TxConfig = state.TxConfig

// Commitment is a 32-byte Fibre blob commitment derived from the blob data.
type Commitment [appfibre.CommitmentSize]byte

// MarshalJSON encodes the commitment as a hex string.
func (c Commitment) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(c[:]))
}

// UnmarshalJSON decodes a hex string into a commitment.
func (c *Commitment) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return fmt.Errorf("invalid commitment hex: %w", err)
	}
	if len(b) != appfibre.CommitmentSize {
		return fmt.Errorf("invalid commitment length: got %d, want %d", len(b), appfibre.CommitmentSize)
	}
	copy(c[:], b)
	return nil
}

// String returns the hex-encoded commitment.
func (c Commitment) String() string {
	return hex.EncodeToString(c[:])
}
