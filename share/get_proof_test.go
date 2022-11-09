package share

import (
	"context"
	"math/rand"
	"strconv"
	"testing"
	"time"

	mdutils "github.com/ipfs/go-merkledag/test"
	"github.com/minio/sha256-simd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share/ipld"
)

func TestGetSharesWithProofsByNamespace(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bServ := mdutils.Bserv()

	var tests = []struct {
		rawData []Share
	}{
		{rawData: RandShares(t, 4)},
		{rawData: RandShares(t, 16)},
		{rawData: RandShares(t, 64)},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			rand.Seed(time.Now().UnixNano())
			// choose random range in shares f
			from := rand.Intn(len(tt.rawData) - 1)
			to := rand.Intn(len(tt.rawData) - 1)

			//
			expected := tt.rawData[from]
			nID := expected[:NamespaceSize]

			if to < from {
				tmp := from
				from, to = to, tmp
			}

			// change rawData to contain several shares with same nID
			for i := from; i <= to; i++ {
				tt.rawData[i] = expected
			}

			// put raw data in BlockService
			eds, err := AddShares(ctx, tt.rawData, bServ)
			require.NoError(t, err)

			var shares []Share
			for _, row := range eds.RowRoots() {
				rcid := ipld.MustCidFromNamespacedSha256(row)
				rowShares, err := GetSharesWithProofsByNamespace(ctx, bServ, rcid, nID, len(eds.RowRoots()))
				require.NoError(t, err)
				if rowShares != nil {
					// append shares to check integrity later
					shares = append(shares, rowShares.Shares...)

					// construct nodes from shares by prepending namespace
					var leafs [][]byte
					for _, sh := range rowShares.Shares {
						leafs = append(leafs, append(sh[:NamespaceSize], sh...))
					}

					// validate proof
					verified := rowShares.Proof.VerifyNamespace(
						sha256.New(),
						nID,
						leafs,
						ipld.NamespacedSha256FromCID(rcid))
					require.True(t, verified)
				}
			}

			// validate shares
			assert.Equal(t, to-from+1, len(shares))
			for _, share := range shares {
				assert.Equal(t, expected, share)
			}
		})
	}
}
