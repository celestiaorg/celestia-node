package blobstream

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/merkle"
)

func TestPadBytes(t *testing.T) {
	tests := []struct {
		input     []byte
		length    int
		expected  []byte
		expectErr bool
	}{
		{input: []byte{1, 2, 3}, length: 5, expected: []byte{0, 0, 1, 2, 3}},
		{input: []byte{1, 2, 3}, length: 3, expected: []byte{1, 2, 3}},
		{input: []byte{1, 2, 3}, length: 2, expected: nil, expectErr: true},
		{input: []byte{}, length: 3, expected: []byte{0, 0, 0}},
	}

	for _, test := range tests {
		result, err := padBytes(test.input, test.length)
		if test.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, test.expected, result)
		}
	}
}

func TestTo32PaddedHexBytes(t *testing.T) {
	tests := []struct {
		number      uint64
		expected    []byte
		expectError bool
	}{
		{
			number: 10,
			expected: func() []byte {
				res, _ := hex.DecodeString("000000000000000000000000000000000000000000000000000000000000000a")
				return res
			}(),
		},
		{
			number: 255,
			expected: func() []byte {
				res, _ := hex.DecodeString("00000000000000000000000000000000000000000000000000000000000000ff")
				return res
			}(),
		},
		{
			number: 255,
			expected: func() []byte {
				res, _ := hex.DecodeString("00000000000000000000000000000000000000000000000000000000000000ff")
				return res
			}(),
		},
		{
			number: 4294967295,
			expected: func() []byte {
				res, _ := hex.DecodeString("00000000000000000000000000000000000000000000000000000000ffffffff")
				return res
			}(),
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("number: %d", test.number), func(t *testing.T) {
			result, err := to32PaddedHexBytes(test.number)
			if test.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, result)
			}
		})
	}
}

func TestEncodeDataRootTuple(t *testing.T) {
	height := uint64(2)
	dataRoot, err := hex.DecodeString("82dc1607d84557d3579ce602a45f5872e821c36dbda7ec926dfa17ebc8d5c013")
	require.NoError(t, err)

	expectedEncoding, err := hex.DecodeString(
		// hex representation of height padded to 32 bytes
		"0000000000000000000000000000000000000000000000000000000000000002" +
			// data root
			"82dc1607d84557d3579ce602a45f5872e821c36dbda7ec926dfa17ebc8d5c013",
	)
	require.NoError(t, err)
	require.NotNil(t, expectedEncoding)

	actualEncoding, err := encodeDataRootTuple(height, *(*[32]byte)(dataRoot))
	require.NoError(t, err)
	require.NotNil(t, actualEncoding)

	// Check that the length of packed data is correct
	assert.Equal(t, len(actualEncoding), 64)
	assert.Equal(t, expectedEncoding, actualEncoding)
}

func TestHashDataRootTuples(t *testing.T) {
	tests := map[string]struct {
		tuples       [][]byte
		expectedHash []byte
		expectErr    bool
	}{
		"empty tuples list": {tuples: nil, expectErr: true},
		"valid list of encoded data root tuples": {
			tuples: func() [][]byte {
				tuple1, _ := encodeDataRootTuple(1, [32]byte{0x1})
				tuple2, _ := encodeDataRootTuple(2, [32]byte{0x2})
				return [][]byte{tuple1, tuple2}
			}(),
			expectedHash: func() []byte {
				tuple1, _ := encodeDataRootTuple(1, [32]byte{0x1})
				tuple2, _ := encodeDataRootTuple(2, [32]byte{0x2})

				return merkle.HashFromByteSlices([][]byte{tuple1, tuple2})
			}(),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := hashDataRootTuples(tc.tuples)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedHash, result)
			}
		})
	}
}

func TestProveDataRootTuples(t *testing.T) {
	tests := map[string]struct {
		tuples        [][]byte
		height        uint64
		rangeStart    uint64
		expectedProof merkle.Proof
		expectErr     bool
	}{
		"empty tuples list":       {tuples: [][]byte{{0x1}}, expectErr: true},
		"start height == 0":       {tuples: [][]byte{{0x1}}, expectErr: true},
		"range start height == 0": {tuples: [][]byte{{0x1}}, expectErr: true},
		"valid proof": {
			height:     3,
			rangeStart: 1,
			tuples: func() [][]byte {
				encodedTuple1, _ := encodeDataRootTuple(1, [32]byte{0x1})
				encodedTuple2, _ := encodeDataRootTuple(2, [32]byte{0x2})
				encodedTuple3, _ := encodeDataRootTuple(3, [32]byte{0x3})
				encodedTuple4, _ := encodeDataRootTuple(4, [32]byte{0x4})
				return [][]byte{encodedTuple1, encodedTuple2, encodedTuple3, encodedTuple4}
			}(),
			expectedProof: func() merkle.Proof {
				encodedTuple1, _ := encodeDataRootTuple(1, [32]byte{0x1})
				encodedTuple2, _ := encodeDataRootTuple(2, [32]byte{0x2})
				encodedTuple3, _ := encodeDataRootTuple(3, [32]byte{0x3})
				encodedTuple4, _ := encodeDataRootTuple(4, [32]byte{0x4})
				_, proofs := merkle.ProofsFromByteSlices([][]byte{encodedTuple1, encodedTuple2, encodedTuple3, encodedTuple4})
				return *proofs[2]
			}(),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := proveDataRootTuples(tc.tuples, tc.rangeStart, tc.height)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedProof, *result)
			}
		})
	}
}
