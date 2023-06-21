package share

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	appns "github.com/celestiaorg/celestia-app/pkg/namespace"
)

var (
	validID = append(
		appns.NamespaceVersionZeroPrefix,
		bytes.Repeat([]byte{1}, appns.NamespaceVersionZeroIDSize)...,
	)
	tooShortID      = append(appns.NamespaceVersionZeroPrefix, []byte{1}...)
	tooLongID       = append(appns.NamespaceVersionZeroPrefix, bytes.Repeat([]byte{1}, NamespaceSize)...)
	invalidPrefixID = bytes.Repeat([]byte{1}, NamespaceSize)
)

func TestFrom(t *testing.T) {
	type testCase struct {
		name    string
		bytes   []byte
		wantErr bool
		want    Namespace
	}
	validNamespace := []byte{}
	validNamespace = append(validNamespace, appns.NamespaceVersionZero)
	validNamespace = append(validNamespace, appns.NamespaceVersionZeroPrefix...)
	validNamespace = append(validNamespace, bytes.Repeat([]byte{0x1}, appns.NamespaceVersionZeroIDSize)...)
	parityNamespace := bytes.Repeat([]byte{0xFF}, NamespaceSize)

	testCases := []testCase{
		{
			name:    "valid namespace",
			bytes:   validNamespace,
			wantErr: false,
			want:    append([]byte{appns.NamespaceVersionZero}, validID...),
		},
		{
			name:    "parity namespace",
			bytes:   parityNamespace,
			wantErr: false,
			want:    append([]byte{appns.NamespaceVersionMax}, bytes.Repeat([]byte{0xFF}, appns.NamespaceIDSize)...),
		},
		{
			name: "unsupported version",
			bytes: append([]byte{1}, append(
				appns.NamespaceVersionZeroPrefix,
				bytes.Repeat([]byte{1}, NamespaceSize-len(appns.NamespaceVersionZeroPrefix))...,
			)...),
			wantErr: true,
		},
		{
			name:    "unsupported id: too short",
			bytes:   append([]byte{appns.NamespaceVersionZero}, tooShortID...),
			wantErr: true,
		},
		{
			name:    "unsupported id: too long",
			bytes:   append([]byte{appns.NamespaceVersionZero}, tooLongID...),
			wantErr: true,
		},
		{
			name:    "unsupported id: invalid prefix",
			bytes:   append([]byte{appns.NamespaceVersionZero}, invalidPrefixID...),
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NamespaceFromBytes(tc.bytes)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
