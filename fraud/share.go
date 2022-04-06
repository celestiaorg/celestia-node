package fraud

import (
	"crypto/sha256"

	"github.com/celestiaorg/nmt"

	pb "github.com/celestiaorg/celestia-node/fraud/pb"
)

type Share struct {
	NamespaceID []byte
	Raw         []byte
	Proof       *nmt.Proof
}

func (s *Share) Validate(root []byte) bool {
	return s.Proof.VerifyInclusion(sha256.New(), s.NamespaceID, s.Raw, root)
}

func (s *Share) ShareToProto() *pb.Share {
	return &pb.Share{
		NamespaceID: s.NamespaceID,
		Raw:         s.Raw,
		Proof: &pb.MerkleProof{
			Start:    int64(s.Proof.Start()),
			End:      int64(s.Proof.End()),
			Nodes:    s.Proof.Nodes(),
			LeafHash: s.Proof.LeafHash(),
		},
	}
}

func ProtoToShare(protoShares []*pb.Share) []*Share {
	shares := make([]*Share, len(protoShares))
	for _, share := range protoShares {
		proof := ProtoToProof(share.Proof)
		shares = append(shares, &Share{share.NamespaceID, share.Raw, &proof})
	}
	return shares
}
