package share

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func TestConfig_Validate_StoreODSOnly(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		tp      node.Type
		wantErr bool
		errMsg  string
	}{
		{
			name: "ODS-only for Bridge node - valid",
			cfg: Config{
				StoreODSOnly: true,
			},
			tp:      node.Bridge,
			wantErr: false,
		},
		{
			name: "ODS-only for Light node - invalid",
			cfg: Config{
				StoreODSOnly: true,
			},
			tp:      node.Light,
			wantErr: true,
			errMsg:  "store.ods_only is only supported for Bridge nodes",
		},
		{
			name: "ODS-only for Full node - invalid",
			cfg: Config{
				StoreODSOnly: true,
			},
			tp:      node.Full,
			wantErr: true,
			errMsg:  "store.ods_only is only supported for Bridge nodes",
		},
		{
			name: "ODS-only disabled - valid for all node types",
			cfg: Config{
				StoreODSOnly: false,
			},
			tp:      node.Bridge,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.EDSStoreParams = DefaultConfig(tt.tp).EDSStoreParams
			tt.cfg.Discovery = DefaultConfig(tt.tp).Discovery
			tt.cfg.ShrexClient = DefaultConfig(tt.tp).ShrexClient
			tt.cfg.ShrexServer = DefaultConfig(tt.tp).ShrexServer
			tt.cfg.PeerManagerParams = DefaultConfig(tt.tp).PeerManagerParams
			if tt.tp == node.Light {
				tt.cfg.LightAvailability = DefaultConfig(tt.tp).LightAvailability
			}

			err := tt.cfg.Validate(tt.tp)
			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
