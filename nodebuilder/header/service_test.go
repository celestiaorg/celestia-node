package header

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/merkle"

	libhead "github.com/celestiaorg/go-header"
	"github.com/celestiaorg/go-header/sync"

	"github.com/celestiaorg/celestia-node/header"
)

func TestGetByHeightHandlesError(t *testing.T) {
	serv := Service{
		syncer: &errorSyncer[*header.ExtendedHeader]{},
	}

	assert.NotPanics(t, func() {
		h, err := serv.GetByHeight(context.Background(), 100)
		assert.Error(t, err)
		assert.Nil(t, h)
	})
}

type errorSyncer[H libhead.Header[H]] struct{}

func (d *errorSyncer[H]) Head(context.Context, ...libhead.HeadOption[H]) (H, error) {
	var zero H
	return zero, fmt.Errorf("dummy error")
}

func (d *errorSyncer[H]) State() sync.State {
	return sync.State{}
}

func (d *errorSyncer[H]) SyncWait(context.Context) error {
	return fmt.Errorf("dummy error")
}

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
			result, err := To32PaddedHexBytes(test.number)
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

	actualEncoding, err := EncodeDataRootTuple(height, *(*[32]byte)(dataRoot))
	require.NoError(t, err)
	require.NotNil(t, actualEncoding)

	// Check that the length of packed data is correct
	assert.Equal(t, len(actualEncoding), 64)
	assert.Equal(t, expectedEncoding, actualEncoding)
}

func TestHashDataRootTuples(t *testing.T) {
	tests := map[string]struct {
		tuples       []DataRootTuple
		expectedHash []byte
		expectErr    bool
	}{
		"empty tuples list": {tuples: nil, expectErr: true},
		"valid list of data root tuples": {
			tuples: []DataRootTuple{
				{
					height:   1,
					dataRoot: [32]byte{0x1},
				},
				{
					height:   2,
					dataRoot: [32]byte{0x2},
				},
			},
			expectedHash: func() []byte {
				tuple1, _ := EncodeDataRootTuple(1, [32]byte{0x1})
				tuple2, _ := EncodeDataRootTuple(2, [32]byte{0x2})

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
		tuples        []DataRootTuple
		height        int64
		expectedProof merkle.Proof
		expectErr     bool
	}{
		"empty tuples list": {tuples: nil, expectErr: true},
		"strictly negative height": {
			height: -1,
			tuples: []DataRootTuple{
				{
					height:   1,
					dataRoot: [32]byte{0x1},
				},
			},
			expectErr: true,
		},
		"non consecutive list of tuples at the beginning": {
			tuples: []DataRootTuple{
				{
					height:   1,
					dataRoot: [32]byte{0x1},
				},
				{
					height:   3,
					dataRoot: [32]byte{0x2},
				},
				{
					height:   4,
					dataRoot: [32]byte{0x4},
				},
			},
			expectErr: true,
		},
		"non consecutive list of tuples in the middle": {
			tuples: []DataRootTuple{
				{
					height:   1,
					dataRoot: [32]byte{0x1},
				},
				{
					height:   2,
					dataRoot: [32]byte{0x2},
				},
				{
					height:   3,
					dataRoot: [32]byte{0x2},
				},
				{
					height:   5,
					dataRoot: [32]byte{0x4},
				},
				{
					height:   6,
					dataRoot: [32]byte{0x5},
				},
			},
			expectErr: true,
		},
		"non consecutive list of tuples at the end": {
			tuples: []DataRootTuple{
				{
					height:   1,
					dataRoot: [32]byte{0x1},
				},
				{
					height:   2,
					dataRoot: [32]byte{0x2},
				},
				{
					height:   4,
					dataRoot: [32]byte{0x4},
				},
			},
			expectErr: true,
		},
		"duplicate height at the beginning": {
			tuples: []DataRootTuple{
				{
					height:   1,
					dataRoot: [32]byte{0x1},
				},
				{
					height:   1,
					dataRoot: [32]byte{0x1},
				},
				{
					height:   4,
					dataRoot: [32]byte{0x4},
				},
			},
			expectErr: true,
		},
		"duplicate height in the middle": {
			tuples: []DataRootTuple{
				{
					height:   1,
					dataRoot: [32]byte{0x1},
				},
				{
					height:   2,
					dataRoot: [32]byte{0x2},
				},
				{
					height:   2,
					dataRoot: [32]byte{0x2},
				},
				{
					height:   3,
					dataRoot: [32]byte{0x3},
				},
			},
			expectErr: true,
		},
		"duplicate height at the end": {
			tuples: []DataRootTuple{
				{
					height:   1,
					dataRoot: [32]byte{0x1},
				},
				{
					height:   2,
					dataRoot: [32]byte{0x2},
				},
				{
					height:   2,
					dataRoot: [32]byte{0x2},
				},
			},
			expectErr: true,
		},
		"valid proof": {
			height: 3,
			tuples: []DataRootTuple{
				{
					height:   1,
					dataRoot: [32]byte{0x1},
				},
				{
					height:   2,
					dataRoot: [32]byte{0x2},
				},
				{
					height:   3,
					dataRoot: [32]byte{0x3},
				},
				{
					height:   4,
					dataRoot: [32]byte{0x4},
				},
			},
			expectedProof: func() merkle.Proof {
				encodedTuple1, _ := EncodeDataRootTuple(1, [32]byte{0x1})
				encodedTuple2, _ := EncodeDataRootTuple(2, [32]byte{0x2})
				encodedTuple3, _ := EncodeDataRootTuple(3, [32]byte{0x3})
				encodedTuple4, _ := EncodeDataRootTuple(4, [32]byte{0x4})
				_, proofs := merkle.ProofsFromByteSlices([][]byte{encodedTuple1, encodedTuple2, encodedTuple3, encodedTuple4})
				return *proofs[2]
			}(),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := proveDataRootTuples(tc.tuples, tc.height)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedProof, *result)
			}
		})
	}
}
