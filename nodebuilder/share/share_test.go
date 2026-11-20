package share

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header"
	headermocks "github.com/celestiaorg/celestia-node/nodebuilder/header/mocks"
	"github.com/celestiaorg/celestia-node/share/shwap"
	gettermocks "github.com/celestiaorg/celestia-node/share/shwap/getters/mock"
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

func TestModuleGetSamples_RequestLimit(t *testing.T) {
	for _, count := range []int{0, 1, 1024, 1025} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			hs := headermocks.NewMockModule(ctrl)
			getter := gettermocks.NewMockGetter(ctrl)
			m := module{hs: hs, getter: getter}
			coords := make([]shwap.SampleCoords, count)
			hdr := &header.ExtendedHeader{}
			want := make([]shwap.Sample, count)
			if count <= 1024 {
				hs.EXPECT().GetByHeight(gomock.Any(), uint64(1)).Return(hdr, nil)
				getter.EXPECT().GetSamples(gomock.Any(), hdr, coords).Return(want, nil)
			}
			got, err := m.GetSamples(context.Background(), 1, coords)
			if count > 1024 {
				require.ErrorContains(t, err, "maximum 1024")
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, want, got)
		})
	}
}
