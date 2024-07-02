package share

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appns "github.com/celestiaorg/go-square/namespace"
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

func TestNewNamespaceV0(t *testing.T) {
	type testCase struct {
		name     string
		subNid   []byte
		expected Namespace
		wantErr  bool
	}
	testCases := []testCase{
		{
			name:   "8 byte id, gets left padded",
			subNid: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
			expected: Namespace{
				0x0,
				0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // filled zeros
				0x0, 0x0, 0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8,
			}, // id with left padding
			wantErr: false,
		},
		{
			name:   "10 byte id, no padding",
			subNid: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x9, 0x10},
			expected: Namespace{
				0x0,                                                                                      // version
				0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, // filled zeros
				0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0x10,
			}, // id
			wantErr: false,
		},
		{
			name:     "11 byte id",
			subNid:   []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x9, 0x10, 0x11},
			expected: []byte{},
			wantErr:  true,
		},
		{
			name:     "nil id",
			subNid:   nil,
			expected: []byte{},
			wantErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewBlobNamespaceV0(tc.subNid)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}
}

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

func TestValidateForBlob(t *testing.T) {
	type testCase struct {
		name    string
		ns      Namespace
		wantErr bool
	}

	validNamespace, err := NewBlobNamespaceV0(bytes.Repeat([]byte{0x1}, appns.NamespaceVersionZeroIDSize))
	require.NoError(t, err)

	testCases := []testCase{
		{
			name:    "valid blob namespace",
			ns:      validNamespace,
			wantErr: false,
		},
		{
			name:    "invalid blob namespace: parity shares namespace",
			ns:      ParitySharesNamespace,
			wantErr: true,
		},
		{
			name:    "invalid blob namespace: tail padding namespace",
			ns:      TailPaddingNamespace,
			wantErr: true,
		},
		{
			name:    "invalid blob namespace: tx namespace",
			ns:      TxNamespace,
			wantErr: true,
		},
		{
			name:    "invalid blob namespace: namespace version max",
			ns:      append([]byte{appns.NamespaceVersionMax}, bytes.Repeat([]byte{0x0}, appns.NamespaceIDSize)...),
			wantErr: true,
		},
		{
			name:    "invalid blob namespace: primary reserved namespace",
			ns:      primaryReservedNamespace(0x10),
			wantErr: true,
		},
		{
			name:    "invalid blob namespace: secondary reserved namespace",
			ns:      secondaryReservedNamespace(0x10),
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.ns.ValidateForBlob()

			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func primaryReservedNamespace(lastByte byte) Namespace {
	result := make([]byte, NamespaceSize)
	result = append(result, appns.NamespaceVersionZero)
	result = append(result, appns.NamespaceVersionZeroPrefix...)
	result = append(result, bytes.Repeat([]byte{0x0}, appns.NamespaceVersionZeroIDSize-1)...)
	result = append(result, lastByte)
	return result
}

func secondaryReservedNamespace(lastByte byte) Namespace {
	result := make([]byte, NamespaceSize)
	result = append(result, appns.NamespaceVersionMax)
	result = append(result, bytes.Repeat([]byte{0xFF}, appns.NamespaceIDSize-1)...)
	result = append(result, lastByte)
	return result
}
