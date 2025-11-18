package p2p

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		tp      node.Type
		wantErr bool
		errMsg  string
	}{
		{
			name: "P2P disabled for Bridge node - valid",
			cfg: Config{
				Disabled: true,
			},
			tp:      node.Bridge,
			wantErr: false,
		},
		{
			name: "P2P disabled for Light node - invalid",
			cfg: Config{
				Disabled: true,
			},
			tp:      node.Light,
			wantErr: true,
			errMsg:  "p2p.disabled is only supported for Bridge nodes",
		},
		{
			name: "P2P disabled for Full node - invalid",
			cfg: Config{
				Disabled: true,
			},
			tp:      node.Full,
			wantErr: true,
			errMsg:  "p2p.disabled is only supported for Bridge nodes",
		},
		{
			name: "P2P enabled - valid",
			cfg: Config{
				Disabled: false,
			},
			tp:      node.Bridge,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

func TestConfig_ValidateWithShareConfig(t *testing.T) {
	// Create a mock share config struct using reflection to avoid import cycle
	type mockShareConfig struct {
		StoreODSOnly bool
	}

	tests := []struct {
		name     string
		cfg      Config
		tp       node.Type
		shareCfg interface{}
		wantErr  bool
		errMsg   string
	}{
		{
			name: "P2P disabled with ODS-only enabled - valid",
			cfg: Config{
				Disabled: true,
			},
			tp: node.Bridge,
			shareCfg: &mockShareConfig{
				StoreODSOnly: true,
			},
			wantErr: false,
		},
		{
			name: "P2P disabled without ODS-only - invalid",
			cfg: Config{
				Disabled: true,
			},
			tp: node.Bridge,
			shareCfg: &mockShareConfig{
				StoreODSOnly: false,
			},
			wantErr: true,
			errMsg:  "p2p.disabled requires share.store_ods_only to be enabled",
		},
		{
			name: "P2P enabled - validation skipped",
			cfg: Config{
				Disabled: false,
			},
			tp: node.Bridge,
			shareCfg: &mockShareConfig{
				StoreODSOnly: false,
			},
			wantErr: false,
		},
		{
			name: "P2P disabled for Light node - invalid",
			cfg: Config{
				Disabled: true,
			},
			tp: node.Light,
			shareCfg: &mockShareConfig{
				StoreODSOnly: true,
			},
			wantErr: true,
			errMsg:  "p2p.disabled is only supported for Bridge nodes",
		},
		{
			name: "P2P disabled with nil share config - no error",
			cfg: Config{
				Disabled: true,
			},
			tp:       node.Bridge,
			shareCfg: nil,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.ValidateWithShareConfig(tt.tp, tt.shareCfg)
			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

