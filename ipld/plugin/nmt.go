package plugin

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"

	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"

	"github.com/celestiaorg/nmt"
	mh "github.com/multiformats/go-multihash"
)

const (
	// Below used multiformats (one codec, one multihash) seem free:
	// https://github.com/multiformats/multicodec/blob/master/table.csv

	// NmtCodec is the codec used for leaf and inner nodes of an Namespaced Merkle Tree.
	NmtCodec = 0x7700

	// NmtCodecName is the name used during registry of the NmtCodec codec
	NmtCodecName = "nmt-node"

	// Sha256Namespace8Flagged is the multihash code used to hash blocks
	// that contain an NMT node (inner and leaf nodes).
	Sha256Namespace8Flagged = 0x7701

	// DagParserFormatName can be used when putting into the IPLD Dag
	DagParserFormatName = "extended-square-row-or-col"

	// Repeated here to avoid a dependency to the wrapping repo as this makes
	// it hard to compile and use the plugin against a local ipfs version.
	// TODO: plugins have config options; make this configurable instead
	namespaceSize = 8
	shareSize     = 256
	// nmtHashSize is the size of a digest created by an NMT in bytes.
	nmtHashSize = 2*namespaceSize + sha256.Size
)

func init() {
	mustRegisterNamespacedCodec(
		Sha256Namespace8Flagged,
		"sha2-256-namespace8-flagged",
		nmtHashSize,
		sumSha256Namespace8Flagged,
	)
	// this should already happen when the plugin is injected but it doesn't for some CI tests
	ipld.DefaultBlockDecoder.Register(NmtCodec, NmtNodeParser)
	// register the codecs in the global maps
	cid.Codecs[NmtCodecName] = NmtCodec
	cid.CodecToStr[NmtCodec] = NmtCodecName
}

func mustRegisterNamespacedCodec(
	codec uint64,
	name string,
	defaultLength int,
	hashFunc mh.HashFunc,
) {
	if _, ok := mh.Codes[codec]; !ok {
		// make sure that the Codec wasn't registered from somewhere different than this plugin already:
		if _, found := mh.Codes[codec]; found {
			panic(fmt.Sprintf("Codec 0x%X is already present: %v", codec, mh.Codes[codec]))
		}
		// add to mh.Codes map first, otherwise mh.RegisterHashFunc would err:
		mh.Codes[codec] = name
		mh.Names[name] = codec
		mh.DefaultLengths[codec] = defaultLength

		if err := mh.RegisterHashFunc(codec, hashFunc); err != nil {
			panic(fmt.Sprintf("could not register hash function: %v", mh.Codes[codec]))
		}
	}
}

// sumSha256Namespace8Flagged is the mh.HashFunc used to hash leaf and inner nodes.
// It is registered as a mh.HashFunc in the go-multihash module.
func sumSha256Namespace8Flagged(data []byte, _length int) ([]byte, error) {
	isLeafData := data[0] == nmt.LeafPrefix
	if isLeafData {
		return nmt.Sha256Namespace8FlaggedLeaf(data[1:]), nil
	}
	return nmt.Sha256Namespace8FlaggedInner(data[1:]), nil
}

// DataSquareRowOrColumnRawInputParser reads the raw shares and extract the IPLD nodes from the NMT tree.
// Note, to parse without any error the input has to be of the form:
//
// <share_0>| ... |<share_numOfShares - 1>
//
// Note while this coredag.DagParser is implemented here so this plugin can be used from
// the commandline, the ipld Nodes will rather be created together with the NMT
// root instead of re-computing it here.
func DataSquareRowOrColumnRawInputParser(r io.Reader, _mhType uint64, _mhLen int) ([]ipld.Node, error) {
	br := bufio.NewReader(r)
	collector := newNodeCollector()

	n := nmt.New(
		sha256.New(),
		nmt.NamespaceIDSize(namespaceSize),
		nmt.NodeVisitor(collector.visit),
	)

	for {
		namespacedLeaf := make([]byte, shareSize+namespaceSize)
		if _, err := io.ReadFull(br, namespacedLeaf); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if err := n.Push(namespacedLeaf); err != nil {
			return nil, err
		}
	}
	// to trigger the collection of nodes:
	_ = n.Root()
	return collector.ipldNodes(), nil
}

// nmtNodeCollector creates and collects ipld.Nodes if inserted into a nmt tree.
// It is mainly used for testing.
type nmtNodeCollector struct {
	nodes []ipld.Node
}

func newNodeCollector() *nmtNodeCollector {
	// extendedRowOrColumnSize is hardcoded here to avoid importing
	const extendedRowOrColumnSize = 2 * 128
	return &nmtNodeCollector{nodes: make([]ipld.Node, 0, extendedRowOrColumnSize)}
}

func (n nmtNodeCollector) ipldNodes() []ipld.Node {
	return n.nodes
}

func (n *nmtNodeCollector) visit(hash []byte, children ...[]byte) {
	cid := MustCidFromNamespacedSha256(hash)
	switch len(children) {
	case 1:
		n.nodes = prependNode(nmtLeafNode{
			cid:  cid,
			Data: children[0],
		}, n.nodes)
	case 2:
		n.nodes = prependNode(nmtNode{
			cid: cid,
			l:   children[0],
			r:   children[1],
		}, n.nodes)
	default:
		panic("expected a binary tree")
	}
}

func prependNode(newNode ipld.Node, nodes []ipld.Node) []ipld.Node {
	nodes = append(nodes, ipld.Node(nil))
	copy(nodes[1:], nodes)
	nodes[0] = newNode
	return nodes
}

func NmtNodeParser(block blocks.Block) (ipld.Node, error) {
	// length of the domain separator for leaf and inner nodes:
	const prefixOffset = 1
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
	domainSeparator := data[:prefixOffset]
	if bytes.Equal(domainSeparator, leafPrefix) {
		return &nmtLeafNode{
			cid:  block.Cid(),
			Data: data[prefixOffset:],
		}, nil
	}
	if bytes.Equal(domainSeparator, innerPrefix) {
		return nmtNode{
			cid: block.Cid(),
			l:   data[prefixOffset : prefixOffset+nmtHashSize],
			r:   data[prefixOffset+nmtHashSize:],
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
	// TODO(ismail): we might want to export these later
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
	buf, err := mh.Encode(namespacedHash, Sha256Namespace8Flagged)
	if err != nil {
		return cid.Undef, err
	}
	return cid.NewCidV1(NmtCodec, buf), nil
}

// MustCidFromNamespacedSha256 is a wrapper around cidFromNamespacedSha256 that panics
// in case of an error. Use with care and only in places where no error should occur.
func MustCidFromNamespacedSha256(hash []byte) cid.Cid {
	cidFromHash, err := CidFromNamespacedSha256(hash)
	if err != nil {
		panic(
			fmt.Sprintf("malformed hash: %s, codec: %v",
				err,
				mh.Codes[Sha256Namespace8Flagged]),
		)
	}
	return cidFromHash
}
