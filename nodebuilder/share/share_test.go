package share

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestModuleGetRange_RejectsInvalidRange verifies that GetRange rejects an
// invalid [start, end) range at the API boundary, before touching any
// dependency. The module is constructed with nil deps on purpose: if the guard
// is removed, GetRange dereferences the nil header service and panics instead
// of returning the validation error.
func TestModuleGetRange_RejectsInvalidRange(t *testing.T) {
	m := module{}
	ctx := context.Background()

	t.Run("negative start", func(t *testing.T) {
		_, err := m.GetRange(ctx, 1, -1, 4)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid range")
	})

	t.Run("start >= end", func(t *testing.T) {
		_, err := m.GetRange(ctx, 1, 5, 3)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid range")
	})
}
