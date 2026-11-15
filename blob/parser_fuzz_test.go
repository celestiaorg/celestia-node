package blob

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v10/pkg/wrapper"
	libshare "github.com/celestiaorg/go-square/v4/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds"
	"github.com/celestiaorg/celestia-node/share/shwap"
)

// namespaceGetter serves GetNamespaceData from a single square, the way the
// shrex getter returns it after verification.
type namespaceGetter struct {
	shwap.Getter
	accessor *eds.Rsmt2D
}

func (g *namespaceGetter) GetNamespaceData(
	ctx context.Context,
	hdr *header.ExtendedHeader,
	ns libshare.Namespace,
) (shwap.NamespaceData, error) {
	rows, err := share.RowsWithNamespace(hdr.DAH, ns)
	if err != nil {
		return nil, err
	}
	nd := make(shwap.NamespaceData, 0, len(rows))
	for _, row := range rows {
		rnd, err := g.accessor.RowNamespaceData(ctx, ns, row)
		if err != nil {
			return nil, err
		}
		nd = append(nd, rnd)
	}
	return nd, nil
}

// FuzzBlobParse builds a square whose namespace holds fuzz-chosen share
// bytes, as a byzantine proposer could, and runs the blob service's share
// parser over it through GetAll and Get. Whatever the shares hold, parsing
// must not panic, and every blob GetAll returns must be retrievable by Get
// with its commitment.
func FuzzBlobParse(f *testing.F) {
	const odsSize = 4
	ns := libshare.MustNewV0Namespace(bytes.Repeat([]byte{0x07}, libshare.NamespaceVersionZeroIDSize))
	payload := libshare.ShareSize - libshare.NamespaceSize

	// Honest seeds: the namespace shares of real blobs of various sizes and versions.
	for _, sizes := range [][]int{{10}, {600}, {10, 2000}, {100, 100, 100}} {
		shares := make([]libshare.Share, 0, len(sizes))
		for _, size := range sizes {
			b, err := NewBlobV0(ns, bytes.Repeat([]byte{0xcd}, size))
			require.NoError(f, err)
			shrs, err := BlobsToShares(b)
			require.NoError(f, err)
			shares = append(shares, shrs...)
		}
		f.Add(sharePayloads(shares))
	}
	signer := bytes.Repeat([]byte{0x01}, libshare.SignerSize)
	b, err := NewBlobV1(ns, bytes.Repeat([]byte{0xef}, 700), signer)
	require.NoError(f, err)
	shrs, err := BlobsToShares(b)
	require.NoError(f, err)
	f.Add(sharePayloads(shrs))
	f.Add(bytes.Repeat([]byte{0xff}, payload))
	f.Add([]byte{0x01})

	f.Fuzz(func(t *testing.T, raw []byte) {
		ctx := context.Background()
		// Turn the input into at most odsSize*odsSize namespace shares and pad
		// the rest of the square with tail padding.
		shares := make([]libshare.Share, 0, odsSize*odsSize)
		for off := 0; off < len(raw) && len(shares) < odsSize*odsSize; off += payload {
			data := make([]byte, libshare.ShareSize)
			copy(data, ns.Bytes())
			copy(data[libshare.NamespaceSize:], raw[off:min(len(raw), off+payload)])
			sh, err := libshare.NewShare(data)
			if err != nil {
				return
			}
			shares = append(shares, sh)
		}
		if len(shares) == 0 {
			return
		}
		for len(shares) < odsSize*odsSize {
			shares = append(shares, libshare.TailPaddingShare())
		}

		square, err := rsmt2d.ComputeExtendedDataSquare(
			libshare.ToBytes(shares), share.DefaultRSMT2DCodec(), wrapper.NewConstructor(odsSize))
		require.NoError(t, err)
		roots, err := share.NewAxisRoots(square)
		require.NoError(t, err)
		hdr := headertest.RandExtendedHeaderWithRoot(t, roots)
		svc := NewService(nil, &namespaceGetter{accessor: &eds.Rsmt2D{ExtendedDataSquare: square}}, nil, nil)

		blobs, err := svc.getBlobs(ctx, ns, hdr)
		if err != nil {
			return
		}
		for _, blob := range blobs {
			require.True(t, blob.Namespace().Equals(ns))
			commitment := blob.Commitment
			sharesParser := &parser{verifyFn: func(b *Blob) bool { return b.compareCommitments(commitment) }}
			got, _, err := svc.retrieveWithHeader(ctx, hdr, ns, sharesParser)
			require.NoError(t, err, "a blob GetAll returns must be retrievable by its commitment")
			require.Equal(t, blob.Data(), got.Data())
		}
	})
}

// sharePayloads concatenates the shares' bytes after their namespaces.
func sharePayloads(shares []libshare.Share) []byte {
	out := make([]byte, 0, len(shares)*(libshare.ShareSize-libshare.NamespaceSize))
	for _, sh := range shares {
		out = append(out, sh.ToBytes()[libshare.NamespaceSize:]...)
	}
	return out
}
