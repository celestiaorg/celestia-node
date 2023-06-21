package share

import (
	"fmt"

	appns "github.com/celestiaorg/celestia-app/pkg/namespace"
	"github.com/celestiaorg/nmt/namespace"
)

// NewNamespaceV0 takes a variable size byte slice and creates a version 0 Namespace ID.
// The byte slice must be <= 10 bytes.
// If it is less than 10 bytes, it will be left padded to size 10 with 0s.
// TODO: Adapt for Namespace in the integration PR
func NewNamespaceV0(subNId []byte) (namespace.ID, error) {
	if lnid := len(subNId); lnid > appns.NamespaceVersionZeroIDSize {
		return nil, fmt.Errorf("namespace id must be <= %v, but it was %v bytes", appns.NamespaceVersionZeroIDSize, lnid)
	}

	id := make([]byte, appns.NamespaceIDSize)
	leftPaddingOffset := appns.NamespaceVersionZeroIDSize - len(subNId)
	copy(id[appns.NamespaceVersionZeroPrefixSize+leftPaddingOffset:], subNId)

	appID, err := appns.New(appns.NamespaceVersionZero, id)
	if err != nil {
		return nil, err
	}

	return appID.Bytes(), nil
}
