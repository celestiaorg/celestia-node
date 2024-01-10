package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/celestiaorg/celestia-node/share"
)

func Test_parseNamespaceID(t *testing.T) {
	type testCase struct {
		name    string
		param   string
		want    share.Namespace
		wantErr bool
	}
	testCases := []testCase{
		{
			param: "0x0c204d39600fddd3",
			name:  "8 byte hex encoded namespace ID gets left padded",
			want: share.Namespace{
				0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
				0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0xc, 0x20, 0x4d, 0x39, 0x60, 0xf, 0xdd, 0xd3,
			},
			wantErr: false,
		},
		{
			name:  "10 byte hex encoded namespace ID",
			param: "0x42690c204d39600fddd3",
			want: share.Namespace{
				0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
				0x0, 0x0, 0x0, 0x0, 0x42, 0x69, 0xc, 0x20, 0x4d, 0x39, 0x60, 0xf, 0xdd, 0xd3,
			},
			wantErr: false,
		},
		{
			name:  "29 byte hex encoded namespace ID",
			param: "0x0000000000000000000000000000000000000001010101010101010101",
			want: share.Namespace{
				0x0,                                                                                      // namespace version
				0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // v0 ID prefix
				0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, // namespace ID
			},
			wantErr: true,
		},
		{
			name:    "11 byte hex encoded namespace ID returns error",
			param:   "0x42690c204d39600fddd3a3",
			want:    share.Namespace{},
			wantErr: true,
		},
		{
			name:  "10 byte base64 encoded namespace ID",
			param: "QmkMIE05YA/d0w==",
			want: share.Namespace{
				0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
				0x0, 0x0, 0x0, 0x0, 0x42, 0x69, 0xc, 0x20, 0x4d, 0x39, 0x60, 0xf, 0xdd, 0xd3,
			},
			wantErr: false,
		},
		{
			name:    "not base64 or hex encoded namespace ID returns error",
			param:   "5748493939429",
			want:    share.Namespace{},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseV0Namespace(tc.param)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func Test_decodeBytes(t *testing.T) {
	type testCase struct {
		name    string
		param   string
		want    []uint8
		wantErr bool
	}
	testCases := []testCase{
		{
			param:   "0x0c204d39600fddd3",
			name:    "8 byte hex encoded namespace ID gets left padded",
			want:    []uint8([]byte{0xc, 0x20, 0x4d, 0x39, 0x60, 0xf, 0xdd, 0xd3}),
			wantErr: false,
		},
		{
			name:    "not base64",
			param:   "Hi blob",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "short hex ecnoded string for blob",
			param:   "0x676d",
			want:    []byte{0x67, 0x6d},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecodeToBytes(tc.param)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
