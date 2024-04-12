package byzantine

import (
	"bytes"
	"errors"
	"fmt"

	logging "github.com/ipfs/go-log/v2"

	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/go-fraud"
	"github.com/celestiaorg/nmt"
	nmt_pb "github.com/celestiaorg/nmt/pb"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	pb "github.com/celestiaorg/celestia-node/share/eds/byzantine/pb"
)

var log = logging.Logger("share/byzantine")

const (
	version = "v0.1"

	BadEncoding fraud.ProofType = "badencoding" + version
)

type BadEncodingProof struct {
	headerHash  []byte
	BlockHeight uint64
	// ShareWithProof contains all shares from row or col.
	// Shares that did not pass verification in rsmt2d will be nil.
	// For non-nil shares MerkleProofs are computed.
	Shares []*share.ShareWithProof
	// Index represents the row/col index where ErrByzantineRow/ErrByzantineColl occurred.
	Index uint32
	// Axis represents the axis that verification failed on.
	Axis rsmt2d.Axis
}

// CreateBadEncodingProof creates a new Bad Encoding Fraud Proof that should be propagated through
// network. The fraud proof will contain shares that did not pass verification and their relevant
// Merkle proofs.
func CreateBadEncodingProof(
	hash []byte,
	height uint64,
	errByzantine *ErrByzantine,
) fraud.Proof[*header.ExtendedHeader] {
	return &BadEncodingProof{
		headerHash:  hash,
		BlockHeight: height,
		Shares:      errByzantine.Shares,
		Index:       errByzantine.Index,
		Axis:        errByzantine.Axis,
	}
}

// Type returns type of fraud proof.
func (p *BadEncodingProof) Type() fraud.ProofType {
	return BadEncoding
}

// HeaderHash returns block hash.
func (p *BadEncodingProof) HeaderHash() []byte {
	return p.headerHash
}

// Height returns block height.
func (p *BadEncodingProof) Height() uint64 {
	return p.BlockHeight
}

// MarshalBinary converts BadEncodingProof to binary.
func (p *BadEncodingProof) MarshalBinary() ([]byte, error) {
	shares := make([]*pb.Share, 0, len(p.Shares))
	for _, share := range p.Shares {
		shares = append(shares, ShareWithProofToProto(share))
	}

	badEncodingFraudProof := pb.BadEncoding{
		HeaderHash: p.headerHash,
		Height:     p.BlockHeight,
		Shares:     shares,
		Index:      p.Index,
		Axis:       pb.Axis(p.Axis),
	}
	return badEncodingFraudProof.Marshal()
}

// UnmarshalBinary converts binary to BadEncodingProof.
func (p *BadEncodingProof) UnmarshalBinary(data []byte) error {
	in := pb.BadEncoding{}
	if err := in.Unmarshal(data); err != nil {
		return err
	}
	axisType := rsmt2d.Axis(in.Axis)
	befp := &BadEncodingProof{
		headerHash:  in.HeaderHash,
		BlockHeight: in.Height,
		Shares:      ProtoToShare(in.Shares, axisType),
		Index:       in.Index,
		Axis:        rsmt2d.Axis(in.Axis),
	}

	*p = *befp

	return nil
}

var (
	errHeightMismatch          = errors.New("height reported in proof does not match with the header's height")
	errIncorrectIndex          = errors.New("row/col index is more then the roots amount")
	errIncorrectAmountOfShares = errors.New("incorrect amount of shares")
	errIncorrectShare          = errors.New("incorrect share received")
	errNMTTreeRootsMatch       = errors.New("recomputed root matches the DAH root")
)

var (
	invalidProofPrefix = fmt.Sprintf("invalid %s proof", BadEncoding)
)

// Validate ensures that fraud proof is correct.
// Validate checks that provided Merkle Proofs correspond to the shares,
// rebuilds bad row or col from received shares, computes Merkle Root
// and compares it with block's Merkle Root.
func (p *BadEncodingProof) Validate(hdr *header.ExtendedHeader) error {
	if hdr.Height() != p.BlockHeight {
		log.Debugf("%s: %s. expected block's height: %d, got: %d",
			invalidProofPrefix,
			errHeightMismatch,
			hdr.Height(),
			p.BlockHeight,
		)
		return errHeightMismatch
	}

	if len(hdr.DAH.RowRoots) != len(hdr.DAH.ColumnRoots) {
		// NOTE: This should never happen as callers of this method should not feed it with a
		// malformed extended header.
		panic(fmt.Sprintf(
			"invalid extended header: length of row and column roots do not match. (rowRoots=%d) (colRoots=%d)",
			len(hdr.DAH.RowRoots),
			len(hdr.DAH.ColumnRoots)),
		)
	}

	// merkleRoots are the roots against which we are going to check the inclusion of the received
	// shares. Changing the order of the roots to prove the shares relative to the orthogonal axis,
	// because inside the rsmt2d library rsmt2d.Row = 0 and rsmt2d.Col = 1
	merkleRoots := hdr.DAH.RowRoots
	if p.Axis == rsmt2d.Row {
		merkleRoots = hdr.DAH.ColumnRoots
	}

	if int(p.Index) >= len(merkleRoots) {
		log.Debugf("%s:%s (%d >= %d)",
			invalidProofPrefix, errIncorrectIndex, int(p.Index), len(merkleRoots),
		)
		return errIncorrectIndex
	}

	if len(p.Shares) != len(merkleRoots) {
		// Since p.Shares should contain all the shares from either a row or a
		// column, it should exactly match the number of row roots. In this
		// context, the number of row roots is the width of the extended data
		// square.
		log.Infof("%s: %s (%d >= %d)",
			invalidProofPrefix, errIncorrectAmountOfShares, int(p.Index), len(merkleRoots),
		)
		return errIncorrectAmountOfShares
	}

	odsWidth := uint64(len(merkleRoots) / 2)
	amount := uint64(0)
	for _, share := range p.Shares {
		if share == nil {
			continue
		}
		amount++
		if amount == odsWidth {
			break
		}
	}

	if amount < odsWidth {
		log.Debugf("%s: %s. not enough shares provided to reconstruct row/col",
			invalidProofPrefix, errIncorrectAmountOfShares)
		return errIncorrectAmountOfShares
	}

	// verify that Merkle proofs correspond to particular shares.
	shares := make([][]byte, len(merkleRoots))
	for index, shr := range p.Shares {
		if shr == nil {
			continue
		}
		// validate inclusion of the share into one of the DAHeader roots
		x, y := index, int(p.Index)
		if p.Axis == rsmt2d.Col {
			x, y = int(p.Index), index
		}
		err := shr.Validate(hdr.DAH, x, y)
		if err != nil {
			log.Debugf("%s: %s at index %d", invalidProofPrefix, errIncorrectShare, index)
			return errIncorrectShare
		}
		shares[index] = shr.Share
	}

	codec := share.DefaultRSMT2DCodec()

	// We can conclude that the proof is valid in case we proved the inclusion of `Shares` but
	// the row/col can't be reconstructed, or the building of NMTree fails.
	rebuiltShares, err := codec.Decode(shares)
	if err != nil {
		log.Debugw("failed to decode shares at height",
			"height", hdr.Height(), "err", err,
		)
		return fmt.Errorf("failed to decode shares: %w", err)
	}

	rebuiltExtendedShares, err := codec.Encode(rebuiltShares[0:odsWidth])
	if err != nil {
		log.Debugw("failed to encode shares at height",
			"height", hdr.Height(), "err", err,
		)
		return fmt.Errorf("failed to encode shares: %w", err)
	}
	copy(rebuiltShares[odsWidth:], rebuiltExtendedShares)

	tree := wrapper.NewErasuredNamespacedMerkleTree(odsWidth, uint(p.Index))
	for _, share := range rebuiltShares {
		err = tree.Push(share)
		if err != nil {
			log.Debugw("failed to build a tree from the reconstructed shares at height",
				"height", hdr.Height(), "err", err,
			)
			return fmt.Errorf("failed to build a tree from the reconstructed shares: %w", err)
		}
	}

	expectedRoot, err := tree.Root()
	if err != nil {
		log.Debugw("failed to build a tree root at height",
			"height", hdr.Height(), "err", err,
		)
		return fmt.Errorf("failed to build a tree root: %w", err)
	}

	// root is a merkle root of the row/col where ErrByzantine occurred
	root := hdr.DAH.RowRoots[p.Index]
	if p.Axis == rsmt2d.Col {
		root = hdr.DAH.ColumnRoots[p.Index]
	}

	// comparing rebuilt Merkle Root of bad row/col with respective Merkle Root of row/col from block.
	if bytes.Equal(expectedRoot, root) {
		log.Debugf("invalid %s proof:%s", BadEncoding, errNMTTreeRootsMatch)
		return errNMTTreeRootsMatch
	}
	return nil
}

func ShareWithProofToProto(s *share.ShareWithProof) *pb.Share {
	if s == nil {
		return &pb.Share{}
	}

	return &pb.Share{
		Data: s.Share,
		Proof: &nmt_pb.Proof{
			Start:                 int64(s.Proof.Start()),
			End:                   int64(s.Proof.End()),
			Nodes:                 s.Proof.Nodes(),
			LeafHash:              s.Proof.LeafHash(),
			IsMaxNamespaceIgnored: s.Proof.IsMaxNamespaceIDIgnored(),
		},
	}
}

func ProtoToShare(protoShares []*pb.Share, proofAxis rsmt2d.Axis) []*share.ShareWithProof {
	shares := make([]*share.ShareWithProof, len(protoShares))
	for i, sh := range protoShares {
		if sh.Proof == nil {
			continue
		}
		proof := ProtoToProof(sh.Proof)
		shares[i] = &share.ShareWithProof{
			Share:     sh.Data,
			Proof:     &proof,
			ProofType: proofAxis,
		}
	}
	return shares
}

func ProtoToProof(protoProof *nmt_pb.Proof) nmt.Proof {
	return nmt.NewInclusionProof(
		int(protoProof.Start),
		int(protoProof.End),
		protoProof.Nodes,
		protoProof.IsMaxNamespaceIgnored,
	)
}
