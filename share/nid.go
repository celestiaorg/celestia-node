package share

import (
	"fmt"

	appns "github.com/celestiaorg/celestia-app/pkg/namespace"
	"github.com/celestiaorg/nmt/namespace"
)

// NewNamespaceV0 takes variable size byte slice anc creates version 0 Namespace ID.
// The bytes slice must be <= 10 bytes.
func NewNamespaceV0(subNId []byte) (namespace.ID, error) {
	if lnid := len(subNId); lnid > appns.NamespaceVersionZeroIDSize {
		return nil, fmt.Errorf("namespace id must be <= %v, but it was %v bytes", appns.NamespaceVersionZeroIDSize, lnid)
	}

	id := make([]byte, 0, appns.NamespaceIDSize)
	id = append(id, appns.NamespaceVersionZeroPrefix...)
	id = append(id, subNId...)
	appID, err := appns.New(appns.NamespaceVersionZero, id)
	if err != nil {
		return nil, err
	}

	return appID.Bytes(), nil
}
