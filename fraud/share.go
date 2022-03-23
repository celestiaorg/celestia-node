package fraud

import (
	"crypto/sha256"

	pb "github.com/celestiaorg/celestia-node/fraud/pb"
	"github.com/celestiaorg/celestia-node/ipld"
	"github.com/celestiaorg/nmt"
)

type Share struct {
	Share []byte
	Proof nmt.Proof
}

func (s *Share) Validate(root []byte) bool {
	return s.Proof.VerifyInclusion(sha256.New(), s.Share[:ipld.NamespaceSize], s.Share[ipld.NamespaceSize:], root)
}

func (s *Share) ShareToProto() *pb.Share {
	share := pb.Share{
		Share: s.Share,
		Proof: &pb.MerkleProof{
			Start:    int64(s.Proof.Start()),
			End:      int64(s.Proof.End()),
			Nodes:    s.Proof.Nodes(),
			LeafHash: s.Proof.LeafHash(),
		},
	}
	return &share
}
