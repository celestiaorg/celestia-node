package byzantine

import (
	"bytes"
	"context"
	"crypto/rand"
	"sort"
	"testing"

	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/stretchr/testify/require"
	core "github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/rsmt2d"
)

func TestFalsePositiveBadEncodingFraudProof(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bServ := mdutils.Bserv()

	squareSize := 8
	ss := generateRandNamespacedRawData(uint32(squareSize*squareSize), 8, 504)

	eds, err := share.AddShares(ctx, ss, bServ)
	require.NoError(t, err)

	dah := da.NewDataAvailabilityHeader(eds)

	// get an arbitrary row
	row := uint(squareSize / 2)
	rowShares := eds.Row(row)
	rowRoot := dah.RowsRoots[row]

	shareProofs, err := GetProofsForShares(ctx, bServ, ipld.MustCidFromNamespacedSha256(rowRoot), rowShares)
	require.NoError(t, err)

	// create a fake error for data that was encoded correctly
	fakeError := ErrByzantine{
		Index:  uint32(row),
		Shares: shareProofs,
		Axis:   rsmt2d.Row,
	}

	h := header.ExtendedHeader{
		RawHeader: core.Header{
			Height: 420,
		},
		DAH: &dah,
		Commit: &core.Commit{
			BlockID: core.BlockID{
				Hash: []byte("made up hash"),
			},
		},
	}

	hhash := h.Hash()

	proof := CreateBadEncodingProof(hhash, uint64(h.Height), &fakeError)

	err = proof.Validate(&h)
	require.Error(t, err)
}

// generateRandNamespacedRawData returns random namespaced raw data for testing purposes.
func generateRandNamespacedRawData(total, nidSize, leafSize uint32) [][]byte {
	data := make([][]byte, total)
	for i := uint32(0); i < total; i++ {
		nid := make([]byte, nidSize)

		_, _ = rand.Read(nid)
		data[i] = nid
	}
	sortByteArrays(data)
	for i := uint32(0); i < total; i++ {
		d := make([]byte, leafSize)

		_, _ = rand.Read(d)
		data[i] = append(data[i], d...)
	}

	return data
}

func sortByteArrays(src [][]byte) {
	sort.Slice(src, func(i, j int) bool { return bytes.Compare(src[i], src[j]) < 0 })
}
