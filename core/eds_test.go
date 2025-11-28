package core

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/availability"
	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/store"
)

func TestStoreEDS_ODSOnly(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		odsOnly  bool
		archival bool
		window   time.Duration
		wantQ4   bool // true if Q4 file should exist, false if only ODS
	}{
		{
			name:     "ODS-only mode - always uses PutODS (no Q4)",
			odsOnly:  true,
			archival: false,
			window:   availability.StorageWindow,
			wantQ4:   false,
		},
		{
			name:     "Normal mode within window - uses PutODSQ4 (has Q4)",
			odsOnly:  false,
			archival: false,
			window:   availability.StorageWindow,
			wantQ4:   true,
		},
		{
			name:     "Normal mode outside window - skips storage (no Q4)",
			odsOnly:  false,
			archival: false,
			window:   time.Nanosecond, // Very small window, header will be outside
			wantQ4:   false,
		},
		{
			name:     "Archival mode within window - uses PutODSQ4 (has Q4)",
			odsOnly:  false,
			archival: true,
			window:   availability.StorageWindow,
			wantQ4:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := t.TempDir()
			testStore, err := store.NewStore(store.DefaultParameters(), testDir)
			require.NoError(t, err)
			defer func() {
				err := testStore.Stop(ctx)
				require.NoError(t, err)
			}()

			eds := edstest.RandEDS(t, 4)
			roots, err := share.NewAxisRoots(eds)
			require.NoError(t, err)

			eh := &header.ExtendedHeader{
				RawHeader: header.RawHeader{
					Height: 1,
					Time:   time.Now(),
				},
				DAH: roots,
			}

			err = storeEDS(ctx, eh, eds, testStore, tt.window, tt.archival, tt.odsOnly)
			require.NoError(t, err)

			if !tt.archival && !availability.IsWithinWindow(eh.Time(), tt.window) {
				has, err := testStore.HasByHeight(ctx, eh.Height())
				require.NoError(t, err)
				require.False(t, has, "Block outside window should not be stored")
				return
			}

			has, err := testStore.HasByHeight(ctx, eh.Height())
			require.NoError(t, err)
			require.True(t, has, "Block should exist in store")

			datahash := share.DataHash(roots.Hash())
			hasQ4, err := testStore.HasQ4ByHash(ctx, datahash)
			require.NoError(t, err)

			if tt.wantQ4 {
				require.True(t, hasQ4, "Expected Q4 file to exist (PutODSQ4 was used), but Q4 file not found")
			} else {
				require.False(t, hasQ4, "Expected no Q4 file (PutODS was used), but Q4 file exists")
			}
		})
	}
}
