package shwap_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	libshare "github.com/celestiaorg/go-square/v4/share"
	nmt_pb "github.com/celestiaorg/nmt/pb"

	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/pb"
)

// oversizedProofNodes returns more nodes than a range proof over a row tree of
// 2*share.MaxSquareSize leaves can ever need, simulating a peer padding the proof
// with cheap empty nodes to inflate the decoder's memory usage.
func oversizedProofNodes() [][]byte {
	nodes := make([][]byte, 64)
	for i := range nodes {
		nodes[i] = []byte{}
	}
	return nodes
}

func TestRowNamespaceDataFromProtoRejectsOversizedProof(t *testing.T) {
	row := &pb.RowNamespaceData{
		Proof: &nmt_pb.Proof{Start: 0, End: 1, Nodes: oversizedProofNodes()},
	}
	_, err := shwap.RowNamespaceDataFromProto(row)
	require.Error(t, err)
}

func TestSampleFromProtoRejectsOversizedProof(t *testing.T) {
	sample := &pb.Sample{
		Share: &pb.Share{Data: make([]byte, libshare.ShareSize)},
		Proof: &nmt_pb.Proof{Start: 0, End: 1, Nodes: oversizedProofNodes()},
	}
	_, err := shwap.SampleFromProto(sample)
	require.Error(t, err)
}

func TestRangeNamespaceDataFromProtoRejectsOversizedProof(t *testing.T) {
	rnd := &pb.RangeNamespaceData{
		Shares: []*pb.RowShares{
			{Shares: []*pb.Share{{Data: make([]byte, libshare.ShareSize)}}},
		},
		FirstIncompleteRowProof: &nmt_pb.Proof{Start: 0, End: 1, Nodes: oversizedProofNodes()},
	}
	_, err := shwap.RangeNamespaceDataFromProto(rnd)
	require.Error(t, err)

	rnd.FirstIncompleteRowProof = nil
	rnd.LastIncompleteRowProof = &nmt_pb.Proof{Start: 0, End: 1, Nodes: oversizedProofNodes()}
	_, err = shwap.RangeNamespaceDataFromProto(rnd)
	require.Error(t, err)
}
