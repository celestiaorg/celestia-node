package ipld

import "github.com/celestiaorg/nmt/namespace"

const (
	// MaxSquareSize is currently the maximum size supported for unerasured data in rsmt2d.ExtendedDataSquare.
	MaxSquareSize = 128
	// ShareSize system wide default size for data shares.
	ShareSize = 256
	// NamespaceSize is a system wide size for NMT namespaces.
	// TODO(Wondertan): Should be part of IPLD/NMT plugin
	NamespaceSize = 8
)

// TODO(Wondertan):
//  Currently Share prepends namespace bytes while NamespaceShare just takes a copy of namespace
//  separating it in separate field. This is really confusing for newcomers and even for those who worked with code,
//  but had some time off of it. Instead, we shouldn't copy(1) and likely have only one type - NamespacedShare, as we
//  don't support shares without namespace.

// Share contains the raw share data without the corresponding namespace.
type Share []byte

// TODO(Wondertan): Consider using alias to namespace.PrefixedData instead
// NamespacedShare extends a Share with the corresponding namespace.
type NamespacedShare struct {
	Share
	ID namespace.ID
}

func (n NamespacedShare) NamespaceID() namespace.ID {
	return n.ID
}

func (n NamespacedShare) Data() []byte {
	return n.Share
}

// NamespacedShares is just a list of NamespacedShare elements.
// It can be used to extract the raw shares.
type NamespacedShares []NamespacedShare

// Raw returns the raw shares that can be fed into the erasure coding
// library (e.g. rsmt2d).
func (ns NamespacedShares) Raw() [][]byte {
	res := make([][]byte, len(ns))
	for i, nsh := range ns {
		res[i] = nsh.Share
	}
	return res
}
