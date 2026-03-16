package share

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"

	cdapb "github.com/celestiaorg/celestia-node/share/cda/pb"
	"github.com/celestiaorg/celestia-node/share/crypto"
)

const (
	cdaMsgTypeStoreRawShare = "cda_store_raw_share_v0"
	cdaMsgTypeFragment      = "cda_fragment_v0"

	// Wire tags for protobuf-framed CDA messages.
	cdaWireTagStoreRawShare byte = 1
	cdaWireTagFragment      byte = 2
)

// CDAStoreMessage distributes a raw share to the custody cell [row,col].
type CDAStoreMessage struct {
	Height uint64
	Row    uint16
	Col    uint16
	Share  []byte
}

// CDAFragmentMessage gossips a coded fragment within a column.
type CDAFragmentMessage struct {
	Height uint64
	Row    uint16
	Col    uint16
	NodeID string

	CodingVec crypto.CodingVector
	Data      []byte
	Proof     crypto.FragmentProof
}

func marshalCDAStoreMessage(m CDAStoreMessage) ([]byte, error) {
	pbMsg := &cdapb.CDAStoreMessage{
		Height: m.Height,
		Row:    uint32(m.Row),
		Col:    uint32(m.Col),
		Share:  m.Share,
	}
	b, err := pbMsg.Marshal()
	if err != nil {
		return nil, err
	}
	return append([]byte{cdaWireTagStoreRawShare}, b...), nil
}

func marshalCDAFragmentMessage(m CDAFragmentMessage) ([]byte, error) {
	seed := m.CodingVec.Seed[:]
	seedNonZero := false
	for _, b := range seed {
		if b != 0 {
			seedNonZero = true
			break
		}
	}

	pbVec := &cdapb.CodingVector{
		K: uint32(m.CodingVec.K),
	}
	if seedNonZero {
		pbVec.Seed = seed
	}
	if len(m.CodingVec.Coeffs) > 0 {
		pbVec.Coeffs = make([][]byte, 0, len(m.CodingVec.Coeffs))
		for _, c := range m.CodingVec.Coeffs {
			pbVec.Coeffs = append(pbVec.Coeffs, c.Bytes)
		}
	}

	pbMsg := &cdapb.CDAFragmentMessage{
		Height:    m.Height,
		Row:       uint32(m.Row),
		Col:       uint32(m.Col),
		PeerId:    m.NodeID,
		CodingVec: pbVec,
		Data:      m.Data,
		Proof:     &cdapb.FragmentProof{ProofBytes: m.Proof.ProofBytes},
	}

	b, err := pbMsg.Marshal()
	if err != nil {
		return nil, err
	}
	return append([]byte{cdaWireTagFragment}, b...), nil
}

func unmarshalCDAMessage(b []byte) (typ string, store *CDAStoreMessage, frag *CDAFragmentMessage, err error) {
	if len(b) < 1 {
		return "", nil, nil, fmt.Errorf("cda: empty message")
	}
	tag := b[0]
	payload := b[1:]

	switch tag {
	case cdaWireTagStoreRawShare:
		var pbMsg cdapb.CDAStoreMessage
		if err := pbMsg.Unmarshal(payload); err != nil {
			return "", nil, nil, err
		}
		m := CDAStoreMessage{
			Height: pbMsg.Height,
			Row:    uint16(pbMsg.Row),
			Col:    uint16(pbMsg.Col),
			Share:  pbMsg.Share,
		}
		return cdaMsgTypeStoreRawShare, &m, nil, nil
	case cdaWireTagFragment:
		var pbMsg cdapb.CDAFragmentMessage
		if err := pbMsg.Unmarshal(payload); err != nil {
			return "", nil, nil, err
		}
		var v crypto.CodingVector
		if pbMsg.CodingVec != nil {
			v.K = uint16(pbMsg.CodingVec.K)
			if len(pbMsg.CodingVec.Seed) == 32 {
				copy(v.Seed[:], pbMsg.CodingVec.Seed)
			}
			if len(pbMsg.CodingVec.Coeffs) > 0 {
				v.Coeffs = make([]crypto.FieldElement, 0, len(pbMsg.CodingVec.Coeffs))
				for _, c := range pbMsg.CodingVec.Coeffs {
					v.Coeffs = append(v.Coeffs, crypto.FieldElement{Bytes: c})
				}
			}
		}
		var proof crypto.FragmentProof
		if pbMsg.Proof != nil {
			proof.ProofBytes = pbMsg.Proof.ProofBytes
		}

		m := CDAFragmentMessage{
			Height:    pbMsg.Height,
			Row:       uint16(pbMsg.Row),
			Col:       uint16(pbMsg.Col),
			NodeID:    pbMsg.PeerId,
			CodingVec: v,
			Data:      pbMsg.Data,
			Proof:     proof,
		}
		return cdaMsgTypeFragment, nil, &m, nil
	default:
		return "", nil, nil, fmt.Errorf("cda: unknown wire tag %d", tag)
	}
}

func (m CDAFragmentMessage) PeerID() (peer.ID, error) {
	return peer.Decode(m.NodeID)
}

