package ipld

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/rand"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
	mh "github.com/multiformats/go-multihash"
	"github.com/tendermint/tendermint/pkg/consts"
	"github.com/tendermint/tendermint/pkg/da"
	"go.opentelemetry.io/otel"

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

	// nmtHashSize is the size of a digest created by an NMT in bytes.
	nmtHashSize = 2*consts.NamespaceSize + sha256.Size

	// MaxSquareSize is currently the maximum size supported for unerasured data in rsmt2d.ExtendedDataSquare.
	MaxSquareSize = consts.MaxSquareSize

	// NamespaceSize is a system-wide size for NMT namespaces.
	NamespaceSize = consts.NamespaceSize

	// cidPrefixSize is the size of the prepended buffer of the CID encoding
	// for NamespacedSha256. For more information, see:
	// https://multiformats.io/multihash/#the-multihash-format
	cidPrefixSize = 4

	// typeSize defines the size of the serialized NMT Node type
	typeSize = 1
)

func GetNode(ctx context.Context, bGetter blockservice.BlockGetter, root cid.Cid) (ipld.Node, error) {
	block, err := bGetter.GetBlock(ctx, root)
	if err != nil {
		var errNotFound *ipld.ErrNotFound
		if errors.As(err, &errNotFound) {
			return nil, errNotFound
		}
		return nil, err
	}

	return decodeBlock(block)
}

func decodeBlock(block blocks.Block) (ipld.Node, error) {
	var (
		leafPrefix  = []byte{nmt.LeafPrefix}
		innerPrefix = []byte{nmt.NodePrefix}
	)
	data := block.RawData()
	if len(data) == 0 {
		return &nmtLeafNode{
			cid:  cid.Undef,
			Data: nil,
		}, nil
	}
	domainSeparator := data[:typeSize]
	if bytes.Equal(domainSeparator, leafPrefix) {
		return &nmtLeafNode{
			cid:  block.Cid(),
			Data: data[typeSize:],
		}, nil
	}
	if bytes.Equal(domainSeparator, innerPrefix) {
		return &nmtNode{
			cid: block.Cid(),
			l:   data[typeSize : typeSize+nmtHashSize],
			r:   data[typeSize+nmtHashSize:],
		}, nil
	}
	return nil, fmt.Errorf(
		"expected first byte of block to be either the leaf or inner node prefix: (%x, %x), got: %x)",
		leafPrefix,
		innerPrefix,
		domainSeparator,
	)
}

var _ ipld.Node = (*nmtNode)(nil)
var _ ipld.Node = (*nmtLeafNode)(nil)

type nmtNode struct {
	cid  cid.Cid
	l, r []byte
}

func NewNMTNode(id cid.Cid, l, r []byte) ipld.Node {
	return nmtNode{id, l, r}
}

func (n nmtNode) RawData() []byte {
	return append([]byte{nmt.NodePrefix}, append(n.l, n.r...)...)
}

func (n nmtNode) Cid() cid.Cid {
	return n.cid
}

func (n nmtNode) String() string {
	return fmt.Sprintf(`
node {
	hash: %x,
	l: %x,
	r: %x"
}`, n.cid.Hash(), n.l, n.r)
}

func (n nmtNode) Loggable() map[string]interface{} {
	return nil
}

func (n nmtNode) Resolve(path []string) (interface{}, []string, error) {
	switch path[0] {
	case "0":
		left, err := CidFromNamespacedSha256(n.l)
		if err != nil {
			return nil, nil, err
		}
		return &ipld.Link{Cid: left}, path[1:], nil
	case "1":
		right, err := CidFromNamespacedSha256(n.r)
		if err != nil {
			return nil, nil, err
		}
		return &ipld.Link{Cid: right}, path[1:], nil
	default:
		return nil, nil, errors.New("invalid path for inner node")
	}
}

func (n nmtNode) Tree(path string, depth int) []string {
	if path != "" || depth != -1 {
		panic("proper tree not yet implemented")
	}

	return []string{
		"0",
		"1",
	}
}

func (n nmtNode) ResolveLink(path []string) (*ipld.Link, []string, error) {
	obj, rest, err := n.Resolve(path)
	if err != nil {
		return nil, nil, err
	}

	lnk, ok := obj.(*ipld.Link)
	if !ok {
		return nil, nil, errors.New("was not a link")
	}

	return lnk, rest, nil
}

func (n nmtNode) Copy() ipld.Node {
	l := make([]byte, len(n.l))
	copy(l, n.l)
	r := make([]byte, len(n.r))
	copy(r, n.r)

	return &nmtNode{
		cid: n.cid,
		l:   l,
		r:   r,
	}
}

func (n nmtNode) Links() []*ipld.Link {
	leftCid := MustCidFromNamespacedSha256(n.l)
	rightCid := MustCidFromNamespacedSha256(n.r)

	return []*ipld.Link{{Cid: leftCid}, {Cid: rightCid}}
}

func (n nmtNode) Stat() (*ipld.NodeStat, error) {
	return &ipld.NodeStat{}, nil
}

func (n nmtNode) Size() (uint64, error) {
	return 0, nil
}

type nmtLeafNode struct {
	cid  cid.Cid
	Data []byte
}

func NewNMTLeafNode(id cid.Cid, data []byte) ipld.Node {
	return &nmtLeafNode{id, data}
}

func (l nmtLeafNode) RawData() []byte {
	return append([]byte{nmt.LeafPrefix}, l.Data...)
}

func (l nmtLeafNode) Cid() cid.Cid {
	return l.cid
}

func (l nmtLeafNode) String() string {
	return fmt.Sprintf(`
leaf {
	hash: 		%x,
	len(Data): 	%v
}`, l.cid.Hash(), len(l.Data))
}

func (l nmtLeafNode) Loggable() map[string]interface{} {
	return nil
}

func (l nmtLeafNode) Resolve(path []string) (interface{}, []string, error) {
	return nil, nil, errors.New("invalid path for leaf node")
}

func (l nmtLeafNode) Tree(_path string, _depth int) []string {
	return nil
}

func (l nmtLeafNode) ResolveLink(path []string) (*ipld.Link, []string, error) {
	obj, rest, err := l.Resolve(path)
	if err != nil {
		return nil, nil, err
	}

	lnk, ok := obj.(*ipld.Link)
	if !ok {
		return nil, nil, errors.New("was not a link")
	}
	return lnk, rest, nil
}

func (l nmtLeafNode) Copy() ipld.Node {
	panic("implement me")
}

func (l nmtLeafNode) Links() []*ipld.Link {
	return []*ipld.Link{{Cid: l.Cid()}}
}

func (l nmtLeafNode) Stat() (*ipld.NodeStat, error) {
	return &ipld.NodeStat{}, nil
}

func (l nmtLeafNode) Size() (uint64, error) {
	return 0, nil
}

// CidFromNamespacedSha256 uses a hash from an nmt tree to create a CID
func CidFromNamespacedSha256(namespacedHash []byte) (cid.Cid, error) {
	if got, want := len(namespacedHash), nmtHashSize; got != want {
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
