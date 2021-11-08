package block

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/celestiaorg/celestia-node/testutils"
)

func Test_validateEncoding(t *testing.T) {
	// generate random raw block data
	rawData, err := testutils.GenerateRandomBlockData(1, 1, 1, 1, 40)
	if err != nil {
		t.Fatal(err)
	}
	// extend the block data
	extendedData, err := extendBlockData(&RawBlock{Data: rawData})
	if err != nil {
		t.Fatal(err)
	}
	// generate DAH
	dah, err := header.DataAvailabilityHeaderFromExtendedData(extendedData)
	if err != nil {
		t.Fatal(err)
	}

	var tests = []struct {
		rawHeader   header.RawHeader
		extended    Block
		errExpected bool
	}{
		// successful case
		{
			rawHeader: header.RawHeader{
				DataHash: dah.Hash(),
			},
			extended: Block{
				header: &header.ExtendedHeader{
					DAH: &dah,
				},
				data: extendedData,
			},
			errExpected: false,
		},
		// malicious case
		{
			rawHeader: header.RawHeader{
				DataHash: make([]byte, len(dah.Hash())),
			},
			extended: Block{
				header: &header.ExtendedHeader{
					DAH: &dah,
				},
				data: extendedData,
			},
			errExpected: true,
		},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			err := validateEncoding(&tt.extended, tt.rawHeader)
			if tt.errExpected {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
