package share

import (
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/minio/sha256-simd"
	"go.opentelemetry.io/otel"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-node/share/ipld"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/nmt/namespace"
)

var (
	tracer = otel.Tracer("share")

	// DefaultRSMT2DCodec sets the default rsmt2d.Codec for shares.
	DefaultRSMT2DCodec = appconsts.DefaultCodec
)

const (
	// MaxSquareSize is currently the maximum size supported for unerasured data in
	// rsmt2d.ExtendedDataSquare.
	MaxSquareSize = appconsts.MaxSquareSize
	// NamespaceSize is a system-wide size for NMT namespaces.
	NamespaceSize = appconsts.NamespaceSize
	// Size is a system-wide size of a share, including both data and namespace ID
	Size = appconsts.ShareSize
)

// Share contains the raw share data without the corresponding namespace.
// NOTE: Alias for the byte is chosen to keep maximal compatibility, especially with rsmt2d.
// Ideally, we should define reusable type elsewhere and make everyone(Core, rsmt2d, ipld) to rely
// on it.
type Share = []byte

// ID gets the namespace ID from the share.
func ID(s Share) namespace.ID {
	return s[:NamespaceSize]
}

// Data gets data from the share.
func Data(s Share) []byte {
	return s[NamespaceSize:]
}

// DataHash is a representation of the Root hash.
type DataHash []byte

func (dh DataHash) Validate() error {
	if len(dh) != 32 {
		return fmt.Errorf("invalid hash size, expected 32, got %d", len(dh))
	}
	return nil
}

func (dh DataHash) String() string {
	return fmt.Sprintf("%X", []byte(dh))
}

type SharesWithProof struct {
	Shares []Share
	Proof  *ipld.Proof
}

// Verify verifies inclusion of the namespaced shares under the given root CID.
func (sp SharesWithProof) Verify(rootCID cid.Cid, nID namespace.ID) bool {
	// construct nodes from shares by prepending namespace
	leaves := make([][]byte, 0, len(sp.Shares))
	for _, sh := range sp.Shares {
		leaves = append(leaves, append(sh[:NamespaceSize], sh...))
	}

	proofNodes := make([][]byte, 0, len(sp.Proof.Nodes))
	for _, n := range sp.Proof.Nodes {
		proofNodes = append(proofNodes, ipld.NamespacedSha256FromCID(n))
	}

	// construct new proof
	inclusionProof := nmt.NewInclusionProof(
		sp.Proof.Start,
		sp.Proof.End,
		proofNodes,
		ipld.NMTIgnoreMaxNamespace)

	// verify inclusion
	return inclusionProof.VerifyNamespace(
		sha256.New(),
		nID,
		leaves,
		ipld.NamespacedSha256FromCID(rootCID))
}
