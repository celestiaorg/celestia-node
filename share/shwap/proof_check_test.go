package shwap

import (
	"testing"

	nmt_pb "github.com/celestiaorg/nmt/pb"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share/shwap/pb"
)

func TestCheckProof(t *testing.T) {
	validNode := make([]byte, nmtNodeSize)
	oversizedNode := make([]byte, 1<<20)

	tooManyNodes := make([][]byte, maxProofNodes+1)
	for i := range tooManyNodes {
		tooManyNodes[i] = validNode
	}

	tests := []struct {
		name    string
		proof   *nmt_pb.Proof
		wantErr string
	}{
		{name: "nil proof", proof: nil},
		{name: "no nodes", proof: &nmt_pb.Proof{}},
		{name: "valid nodes", proof: &nmt_pb.Proof{Nodes: [][]byte{validNode, validNode}}},
		{name: "valid leaf hash", proof: &nmt_pb.Proof{Nodes: [][]byte{validNode}, LeafHash: validNode}},
		{name: "too many nodes", proof: &nmt_pb.Proof{Nodes: tooManyNodes}, wantErr: "too many nodes"},
		{name: "empty node", proof: &nmt_pb.Proof{Nodes: [][]byte{validNode, {}}}, wantErr: "proof node 1 has size 0"},
		{name: "short node", proof: &nmt_pb.Proof{Nodes: [][]byte{validNode[:nmtNodeSize-1]}}, wantErr: "proof node 0 has size"},
		{name: "oversized node", proof: &nmt_pb.Proof{Nodes: [][]byte{oversizedNode}}, wantErr: "proof node 0 has size"},
		{
			name:    "oversized leaf hash",
			proof:   &nmt_pb.Proof{Nodes: [][]byte{validNode}, LeafHash: oversizedNode},
			wantErr: "proof leaf hash has size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkProof(tt.proof)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// Every proof decoded from a peer must go through checkProof.
func TestFromProtoRejectsOversizedProofNode(t *testing.T) {
	oversizedProof := &nmt_pb.Proof{Nodes: [][]byte{make([]byte, 1<<20)}}

	_, err := RowNamespaceDataFromProto(&pb.RowNamespaceData{Proof: oversizedProof})
	require.ErrorContains(t, err, "proof node 0 has size")

	_, err = SampleFromProto(&pb.Sample{Proof: oversizedProof})
	require.ErrorContains(t, err, "proof node 0 has size")

	_, err = RangeNamespaceDataFromProto(&pb.RangeNamespaceData{FirstIncompleteRowProof: oversizedProof})
	require.ErrorContains(t, err, "proof node 0 has size")

	_, err = RangeNamespaceDataFromProto(&pb.RangeNamespaceData{LastIncompleteRowProof: oversizedProof})
	require.ErrorContains(t, err, "proof node 0 has size")
}
