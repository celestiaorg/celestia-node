package ipld

import "fmt"

var (
	ErrNotFoundInRange = fmt.Errorf("namespaceID not found in range")
	ErrBelowRange      = fmt.Errorf("namespaceID is below minimum namespace ID in range")
	ErrExceedsRange    = fmt.Errorf("namespaceID exceeds maximum namespace ID in range")
)
