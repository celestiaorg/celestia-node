package ipld

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"math/rand"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
	mh "github.com/multiformats/go-multihash"
	mhcore "github.com/multiformats/go-multihash/core"
	"go.opentelemetry.io/otel"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/nmt"
)

var (
	tracer = otel.Tracer("ipld")
	log    = logging.Logger("ipld")
)

const (
	// Below used multiformats (one codec, one multihash) seem free:
	// https://github.com/multiformats/multicodec/blob/master/table.csv

	// nmtCodec is the codec used for leaf and inner nodes of a Namespaced Merkle Tree.
	nmtCodec = 0x7700

	// sha256Namespace8Flagged is the multihash code used to hash blocks
	// that contain an NMT node (inner and leaf nodes).
	sha256Namespace8Flagged = 0x7701

	// MaxSquareSize is currently the maximum size supported for unerasured data in
	// rsmt2d.ExtendedDataSquare.
	MaxSquareSize = appconsts.DefaultMaxSquareSize

	// NamespaceSize is a system-wide size for NMT namespaces.
	NamespaceSize = appconsts.NamespaceSize

	// NmtHashSize is the size of a digest created by an NMT in bytes.
	NmtHashSize = 2*NamespaceSize + sha256.Size

	// innerNodeSize is the size of data in inner nodes.
	innerNodeSize = NmtHashSize * 2

	// leafNodeSize is the size of data in leaf nodes.
	leafNodeSize = NamespaceSize + appconsts.ShareSize

	// cidPrefixSize is the size of the prepended buffer of the CID encoding
	// for NamespacedSha256. For more information, see:
	// https://multiformats.io/multihash/#the-multihash-format
	cidPrefixSize = 4

	// NMTIgnoreMaxNamespace is currently used value for IgnoreMaxNamespace option in NMT.
	// IgnoreMaxNamespace defines whether the largest possible namespace.ID MAX_NID should be 'ignored'.
	// If set to true, this allows for shorter proofs in particular use-cases.
	NMTIgnoreMaxNamespace = true
)

func init() {
	// required for Bitswap to hash and verify inbound data correctly
	mhcore.Register(sha256Namespace8Flagged, func() hash.Hash {
		nh := nmt.NewNmtHasher(sha256.New(), NamespaceSize, true)
		nh.Reset()
		return nh
	})
}

func GetNode(ctx context.Context, bGetter blockservice.BlockGetter, root cid.Cid) (ipld.Node, error) {
	block, err := bGetter.GetBlock(ctx, root)
	if err != nil {
		var errNotFound *ipld.ErrNotFound
		if errors.As(err, &errNotFound) {
			return nil, errNotFound
		}
		return nil, err
	}

	return nmtNode{Block: block}, nil
}

type nmtNode struct {
	blocks.Block
}

func newNMTNode(id cid.Cid, data []byte) nmtNode {
	b, err := blocks.NewBlockWithCid(data, id)
	if err != nil {
		panic(fmt.Sprintf("wrong hash for block, cid: %s", id.String()))
	}
	return nmtNode{Block: b}
}

func (n nmtNode) Copy() ipld.Node {
	d := make([]byte, len(n.RawData()))
	copy(d, n.RawData())
	return newNMTNode(n.Cid(), d)
}

func (n nmtNode) Links() []*ipld.Link {
	switch len(n.RawData()) {
	default:
		panic(fmt.Sprintf("unexpected size %v", len(n.RawData())))
	case innerNodeSize:
		leftCid := MustCidFromNamespacedSha256(n.RawData()[:NmtHashSize])
		rightCid := MustCidFromNamespacedSha256(n.RawData()[NmtHashSize:])

		return []*ipld.Link{{Cid: leftCid}, {Cid: rightCid}}
	case leafNodeSize:
		return nil
	}
}

func (n nmtNode) Resolve(path []string) (interface{}, []string, error) {
	panic("method not implemented")
}

func (n nmtNode) Tree(path string, depth int) []string {
	panic("method not implemented")
}

func (n nmtNode) ResolveLink(path []string) (*ipld.Link, []string, error) {
	panic("method not implemented")
}

func (n nmtNode) Stat() (*ipld.NodeStat, error) {
	panic("method not implemented")
}

func (n nmtNode) Size() (uint64, error) {
	panic("method not implemented")
}

// CidFromNamespacedSha256 uses a hash from an nmt tree to create a CID
func CidFromNamespacedSha256(namespacedHash []byte) (cid.Cid, error) {
	if got, want := len(namespacedHash), NmtHashSize; got != want {
		return cid.Cid{}, fmt.Errorf("invalid namespaced hash length, got: %v, want: %v", got, want)
	}
	buf, err := mh.Encode(namespacedHash, sha256Namespace8Flagged)
	if err != nil {
		return cid.Undef, err
	}
	return cid.NewCidV1(nmtCodec, buf), nil
}

// MustCidFromNamespacedSha256 is a wrapper around cidFromNamespacedSha256 that panics
// in case of an error. Use with care and only in places where no error should occur.
func MustCidFromNamespacedSha256(hash []byte) cid.Cid {
	cidFromHash, err := CidFromNamespacedSha256(hash)
	if err != nil {
		panic(
			fmt.Sprintf("malformed hash: %s, codec: %v",
				err,
				mh.Codes[sha256Namespace8Flagged]),
		)
	}
	return cidFromHash
}

// Translate transforms square coordinates into IPLD NMT tree path to a leaf node.
// It also adds randomization to evenly spread fetching from Rows and Columns.
func Translate(dah *da.DataAvailabilityHeader, row, col int) (cid.Cid, int) {
	if rand.Intn(2) == 0 { //nolint:gosec
		return MustCidFromNamespacedSha256(dah.ColumnRoots[col]), row
	}

	return MustCidFromNamespacedSha256(dah.RowsRoots[row]), col
}

// NamespacedSha256FromCID derives the Namespaced hash from the given CID.
func NamespacedSha256FromCID(cid cid.Cid) []byte {
	return cid.Hash()[cidPrefixSize:]
}
