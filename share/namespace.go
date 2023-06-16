package share

import (
	"bytes"
	"encoding/hex"
	"fmt"

	appns "github.com/celestiaorg/celestia-app/pkg/namespace"
	"github.com/celestiaorg/nmt/namespace"
)

// Various reserved namespaces.
var (
	MaxReservedNamespace     = appns.MaxReservedNamespace.Bytes()
	ParitySharesNamespace    = appns.ParitySharesNamespace.Bytes()
	TailPaddingNamespace     = appns.TailPaddingNamespace.Bytes()
	ReservedPaddingNamespace = appns.ReservedPaddingNamespace.Bytes()
	TxNamespace              = appns.TxNamespace.Bytes()
	PayForBlobNamespace      = appns.PayForBlobNamespace.Bytes()
)

// Namespace represents namespace of a Share.
// Consists of version byte and namespace ID.
type Namespace []byte

// NamespaceFromBytes converts bytes into Namespace and validates it.
func NamespaceFromBytes(b []byte) (Namespace, error) {
	n := Namespace(b)
	return n, n.Validate()
}

// Version reports version of the Namespace.
func (n Namespace) Version() byte {
	return n[appns.NamespaceVersionSize-1]
}

// ID reports ID of the Namespace.
func (n Namespace) ID() namespace.ID {
	return namespace.ID(n[appns.NamespaceVersionSize:])
}

// AsNMT converts the whole Namespace(both Version and ID parts) into NMT's namespace.ID
// NOTE: Once https://github.com/celestiaorg/nmt/issues/206 is closed Namespace should become NNT's
// type.
func (n Namespace) AsNMT() namespace.ID {
	return namespace.ID(n)
}

// AsAppNamespace converts the Namespace to App's definition of Namespace.
// TODO: Unify types between node and app
func (n Namespace) AsAppNamespace() appns.Namespace {
	return appns.Namespace{Version: n.Version(), ID: n.ID()}
}

// Len reports the total length of the namespace.
func (n Namespace) Len() int {
	return len(n)
}

// String stringifies the Namespace.
func (n Namespace) String() string {
	return hex.EncodeToString(n)
}

// Validate checks if the namespace is correct.
func (n Namespace) Validate() error {
	if n.Len() != NamespaceSize {
		return fmt.Errorf("invalid namespace length: expected %d, got %d", NamespaceSize, n.Len())
	}
	if n.Version() != appns.NamespaceVersionZero && n.Version() != appns.NamespaceVersionMax {
		return fmt.Errorf("invalid namespace version %v", n.Version())
	}
	if len(n.ID()) != appns.NamespaceIDSize {
		return fmt.Errorf("invalid namespace id length: expected %d, got %d", appns.NamespaceIDSize, n.ID().Size())
	}
	if n.Version() == appns.NamespaceVersionZero && !bytes.HasPrefix(n.ID(), appns.NamespaceVersionZeroPrefix) {
		return fmt.Errorf("invalid namespace id: expect %d leading zeroes", len(appns.NamespaceVersionZeroPrefix))
	}
	return nil
}

// ValidateDataNamespace checks if the Namespace contains real/useful data.
func (n Namespace) ValidateDataNamespace() error {
	if err := n.Validate(); err != nil {
		return err
	}
	if n.IsParityShares() || n.IsTailPadding() {
		return fmt.Errorf("invalid data namespace(%s): parity and tail padding namespace are fobidden", n)
	}
	return nil
}

// ValidateBlobNamespace checks if the Namespace is valid blob namespace.
func (n Namespace) ValidateBlobNamespace() error {
	if err := n.ValidateDataNamespace(); err != nil {
		return err
	}
	if n.IsReserved() {
		return fmt.Errorf("invalid blob namespace(%s): reserved namespaces are forbidden", n)
	}
	return nil
}

// IsAboveMax checks if the namespace is above the maximum namespace of the given hash.
func (n Namespace) IsAboveMax(nodeHash []byte) bool {
	return !n.AsNMT().LessOrEqual(nodeHash[n.Len() : n.Len()*2])
}

// IsBelowMin checks if the target namespace is below the minimum namespace of the given hash.
func (n Namespace) IsBelowMin(nodeHash []byte) bool {
	return n.AsNMT().Less(nodeHash[:n.Len()])
}

// IsOutsideRange checks if the namespace is outside the min-max range of the given hashes.
func (n Namespace) IsOutsideRange(leftNodeHash, rightNodeHash []byte) bool {
	return n.IsBelowMin(leftNodeHash) || n.IsAboveMax(rightNodeHash)
}

func (n Namespace) IsReserved() bool {
	return bytes.Compare(n, MaxReservedNamespace) < 1
}

func (n Namespace) IsParityShares() bool {
	return bytes.Equal(n, ParitySharesNamespace)
}

func (n Namespace) IsTailPadding() bool {
	return bytes.Equal(n, TailPaddingNamespace)
}

func (n Namespace) IsReservedPadding() bool {
	return bytes.Equal(n, ReservedPaddingNamespace)
}

func (n Namespace) IsTx() bool {
	return bytes.Equal(n, TxNamespace)
}

func (n Namespace) IsPayForBlob() bool {
	return bytes.Equal(n, PayForBlobNamespace)
}

func (n Namespace) Repeat(times int) []Namespace {
	ns := make([]Namespace, times)
	for i := 0; i < times; i++ {
		ns[i] = n
	}
	return ns
}

func (n Namespace) Equals(n2 Namespace) bool {
	return bytes.Equal(n, n2)
}

func (n Namespace) IsLessThan(n2 Namespace) bool {
	return bytes.Compare(n, n2) == -1
}

func (n Namespace) IsLessOrEqualThan(n2 Namespace) bool {
	return bytes.Compare(n, n2) < 1
}

func (n Namespace) IsGreaterThan(n2 Namespace) bool {
	return bytes.Compare(n, n2) == 1
}

func (n Namespace) IsGreaterOrEqualThan(n2 Namespace) bool {
	return bytes.Compare(n, n2) > -1
}
