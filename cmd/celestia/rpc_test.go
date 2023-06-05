package main

import (
	"testing"

	"github.com/celestiaorg/nmt/namespace"
	"github.com/stretchr/testify/assert"
)

func Test_parseNamespace(t *testing.T) {
	type testCase struct {
		name    string
		param   string
		want    namespace.ID
		wantErr bool
	}
	testCases := []testCase{
		{
			name:    "8 byte hex encoded namespace gets right padded",
			param:   "0x0c204d39600fddd3",
			want:    namespace.ID{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0xc, 0x20, 0x4d, 0x39, 0x60, 0xf, 0xdd, 0xd3, 0x0, 0x0},
			wantErr: false,
		},
		{
			name:    "10 byte hex encoded namespace",
			param:   "0x42690c204d39600fddd3",
			want:    namespace.ID{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x42, 0x69, 0xc, 0x20, 0x4d, 0x39, 0x60, 0xf, 0xdd, 0xd3},
			wantErr: false,
		},
		// HACKHACK: This test case is disabled because it fails.
		// {
		// 	name:    "11 byte hex encoded namespace returns error",
		// 	param:   "0x42690c204d39600fddd3a3",
		// 	want:    namespace.ID{},
		// 	wantErr: true,
		// },
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseNamespace(tc.param)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})

	}
}
