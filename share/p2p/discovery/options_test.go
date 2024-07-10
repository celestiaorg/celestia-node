package discovery

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		params  Parameters
		wantErr bool
	}{
		{
			name: "valid parameters",
			params: Parameters{
				PeersLimit:            5,
				AdvertiseInterval:     time.Hour,
				AdvertiseRetryTimeout: time.Second,
				DiscoveryRetryTimeout: time.Minute,
			},
			wantErr: false,
		},
		{
			name: "negative PeersLimit",
			params: Parameters{
				PeersLimit:            0,
				AdvertiseInterval:     time.Hour,
				AdvertiseRetryTimeout: time.Second,
				DiscoveryRetryTimeout: time.Minute,
			},
			wantErr: true,
		},
		{
			name: "negative AdvertiseInterval",
			params: Parameters{
				PeersLimit:            5,
				AdvertiseInterval:     -time.Hour,
				AdvertiseRetryTimeout: time.Second,
				DiscoveryRetryTimeout: time.Minute,
			},
			wantErr: true,
		},
		{
			name: "negative AdvertiseRetryTimeout",
			params: Parameters{
				PeersLimit:            5,
				AdvertiseInterval:     time.Hour,
				AdvertiseRetryTimeout: -time.Second,
				DiscoveryRetryTimeout: time.Minute,
			},
			wantErr: true,
		},
		{
			name: "negative DiscoveryRetryTimeout",
			params: Parameters{
				PeersLimit:            5,
				AdvertiseInterval:     time.Hour,
				AdvertiseRetryTimeout: time.Second,
				DiscoveryRetryTimeout: -time.Minute,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.params.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
