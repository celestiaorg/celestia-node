package share

import (
	"bytes"
	"encoding/binary"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/celestiaorg/celestia-node/share/crypto"
)

// FragmentID uniquely identifies a CDA fragment for a particular share
// coordinate (row, col) at a given block height and producing node.
// Row and Col are stored as uint16 to keep the on-disk and on-wire
// footprint small while supporting realistic grid sizes.
type FragmentID struct {
	Height uint64
	Row    uint16
	Col    uint16
	NodeID peer.ID
}

// CanonicalKey returns a stable, lexicographically ordered key suitable
// for KV stores such as BadgerDB.
//
// Layout (bytes):
//   prefix:    []byte("cda:")
//   height:    8 bytes big-endian
//   col:       2 bytes big-endian
//   row:       2 bytes big-endian
//   nodeID:    raw peer.ID bytes (string form)
func (id FragmentID) CanonicalKey() []byte {
	var buf bytes.Buffer
	_, _ = buf.Write([]byte("cda:"))

	var tmp [8]byte
	binary.BigEndian.PutUint64(tmp[:], id.Height)
	_, _ = buf.Write(tmp[:])

	var coord [2]byte
	binary.BigEndian.PutUint16(coord[:], id.Col)
	_, _ = buf.Write(coord[:])
	binary.BigEndian.PutUint16(coord[:], id.Row)
	_, _ = buf.Write(coord[:])

	// peer.ID is a string-like type; use its string bytes as suffix.
	_, _ = buf.Write([]byte(id.NodeID))
	return buf.Bytes()
}

// ParseFragmentKey reverses CanonicalKey back into a FragmentID.
// It expects the same layout as CanonicalKey and returns an error if
// the key is malformed.
func ParseFragmentKey(key []byte) (FragmentID, error) {
	const prefix = "cda:"
	if len(key) < len(prefix)+8+2+2 {
		return FragmentID{}, ErrInvalidFragmentKey
	}
	if string(key[:len(prefix)]) != prefix {
		return FragmentID{}, ErrInvalidFragmentKey
	}

	offset := len(prefix)
	height := binary.BigEndian.Uint64(key[offset : offset+8])
	offset += 8

	col := binary.BigEndian.Uint16(key[offset : offset+2])
	offset += 2

	row := binary.BigEndian.Uint16(key[offset : offset+2])
	offset += 2

	nodeBytes := key[offset:]
	pid, err := peer.Decode(string(nodeBytes))
	if err != nil {
		return FragmentID{}, err
	}

	return FragmentID{
		Height: height,
		Row:    row,
		Col:    col,
		NodeID: pid,
	}, nil
}

// RawShare is the un-coded share payload before fragmentation.
type RawShare []byte

// Fragment is a single coded fragment y = sum g_i x_i, together with the
// coding vector and its homomorphic KZG proof.
type Fragment struct {
	ID        FragmentID
	CodingVec crypto.CodingVector
	Data      []byte
	Proof     crypto.FragmentProof
}

