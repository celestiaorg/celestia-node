package fraud

import (
	"crypto/sha256"

	"github.com/celestiaorg/nmt"

	pb "github.com/celestiaorg/celestia-node/fraud/pb"
)

type ShareWithProof struct {
	NamespaceID []byte
	Data        []byte
	Proof       *nmt.Proof
}

func (s *ShareWithProof) Validate(root []byte) bool {
	// As nmt is prepended NamespaceID twice, we need to pass the full data with NamespaceID in
	// VerifyInclusion
	return s.Proof.VerifyInclusion(sha256.New(), s.NamespaceID, s.Data, root)
}

func (s *ShareWithProof) ShareWithProofToProto() *pb.Share {
	return &pb.Share{
		NamespaceID: s.NamespaceID,
		Data:        s.Data,
		Proof: &pb.MerkleProof{
			Start:    int64(s.Proof.Start()),
			End:      int64(s.Proof.End()),
			Nodes:    s.Proof.Nodes(),
			LeafHash: s.Proof.LeafHash(),
		},
	}
}

func ProtoToShare(protoShares []*pb.Share) []*ShareWithProof {
	shares := make([]*ShareWithProof, len(protoShares))
	for _, share := range protoShares {
		proof := ProtoToProof(share.Proof)
		shares = append(shares, &ShareWithProof{share.NamespaceID, share.Data, &proof})
	}
	return shares
}
