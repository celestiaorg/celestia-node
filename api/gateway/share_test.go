package gateway

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
)

func Test_dataFromShares(t *testing.T) {
	type testCase struct {
		name    string
		input   [][]byte
		want    [][]byte
		wantErr bool
	}

	smallTxInput := padShare([]uint8{
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1, // namespace id
		0x1,                // info byte
		0x0, 0x0, 0x0, 0x2, // 1 byte (unit) + 1 byte (unit length) = 2 bytes sequence length
		0x0, 0x0, 0x0, 17, // reserved bytes
		0x1, // unit length of first transaction
		0xa, // data of first transaction
	})
	smallTxData := []byte{0x1, 0xa}

	largeTxInput := [][]byte{
		fillShare([]uint8{
			0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1, // namespace id
			0x1,                // info byte
			0x0, 0x0, 0x2, 0x2, // 512 (unit) + 2 (unit length) = 514 sequence length
			0x0, 0x0, 0x0, 17, // reserved bytes
			128, 4, // unit length of transaction is 512
		}, 0xc), // data of transaction
		padShare(append([]uint8{
			0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1, // namespace id
			0x0,                // info byte
			0x0, 0x0, 0x0, 0x0, // reserved bytes
		}, bytes.Repeat([]byte{0xc}, 19)..., // continuation data of transaction
		)),
	}
	largeTxData := []byte{128, 4}
	largeTxData = append(largeTxData, bytes.Repeat([]byte{0xc}, 512)...)

	largePfbTxInput := [][]byte{
		fillShare([]uint8{
			0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x4, // namespace id
			0x1,                // info byte
			0x0, 0x0, 0x2, 0x2, // 512 (unit) + 2 (unit length) = 514 sequence length
			0x0, 0x0, 0x0, 17, // reserved bytes
			128, 4, // unit length of transaction is 512
		}, 0xc), // data of transaction
		padShare(append([]uint8{
			0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x4, // namespace id
			0x0,                // info byte
			0x0, 0x0, 0x0, 0x0, // reserved bytes
		}, bytes.Repeat([]byte{0xc}, 19)..., // continuation data of transaction
		)),
	}
	largePfbTxData := []byte{128, 4}
	largePfbTxData = append(largePfbTxData, bytes.Repeat([]byte{0xc}, 512)...)

	testCases := []testCase{
		{
			name:    "empty",
			input:   [][]byte{},
			want:    nil,
			wantErr: false,
		},
		{
			name: "returns an error when shares contain two different namespaces",
			input: [][]byte{
				{0, 0, 0, 0, 0, 0, 0, 1},
				{0, 0, 0, 0, 0, 0, 0, 2},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "returns raw data of a single tx share",
			input:   [][]byte{smallTxInput},
			want:    [][]byte{smallTxData},
			wantErr: false,
		},
		{
			name:    "returns raw data of a large tx that spans two shares",
			input:   largeTxInput,
			want:    [][]byte{largeTxData},
			wantErr: false,
		},
		{
			name:    "returns raw data of a large PFB tx that spans two shares",
			input:   largePfbTxInput,
			want:    [][]byte{largePfbTxData},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := dataFromShares(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// padShare returns a share padded with trailing zeros.
func padShare(share []byte) (paddedShare []byte) {
	return fillShare(share, 0)
}

// fillShare returns a share filled with filler so that the share length
// is equal to appconsts.ShareSize.
func fillShare(share []byte, filler byte) (paddedShare []byte) {
	return append(share, bytes.Repeat([]byte{filler}, appconsts.ShareSize-len(share))...)
}

// sharesBase64JSON is the base64 encoded share data from Blockspace Race
// block height 559108 and namespace e8e5f679bf7116cb.
//
//go:embed "testdata/sharesBase64.json"
var sharesBase64JSON string

// Test_dataFromSharesBSR reproduces an error that occurred when parsing shares
// on Blockspace Race block height 559108 namespace e8e5f679bf7116cb.
//
// https://github.com/celestiaorg/celestia-app/issues/1816
func Test_dataFromSharesBSR(t *testing.T) {
	var sharesBase64 []string
	err := json.Unmarshal([]byte(sharesBase64JSON), &sharesBase64)
	assert.NoError(t, err)
	input := decode(sharesBase64)

	_, err = dataFromShares(input)
	assert.NoError(t, err)
}

// decode returns the raw shares from base64Encoded.
func decode(base64Encoded []string) (rawShares [][]byte) {
	for _, share := range base64Encoded {
		rawShare, err := base64.StdEncoding.DecodeString(share)
		if err != nil {
			panic(err)
		}
		rawShares = append(rawShares, rawShare)
	}
	return rawShares
}
