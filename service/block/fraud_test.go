package block

import (
	"testing"

	"github.com/celestiaorg/celestia-core/testutils"
	"github.com/celestiaorg/celestia-node/service/header"
	"github.com/stretchr/testify/require"
)

func Test_validateEncoding_Successful(t *testing.T) {
	data, err := testutils.GenerateRandomBlockData(1, 1, 1, 1, 40)
	if err != nil {
		t.Fatal(err)
	}
	// create raw block for extending
	rawBlock := &RawBlock{
		Data: data,
	}
	// extend the raw block to be able to generate the DAH
	extendedData, err := extendBlockData(rawBlock)
	if err != nil {
		t.Fatal(err)
	}
	// generate the DAH from the extended block data
	dah, err := header.DataAvailabilityHeaderFromExtendedData(extendedData)
	if err != nil {
		t.Fatal(err)
	}
	// use DAH hash for raw block header
	rawBlock.Header = header.RawHeader{
		DataHash: dah.Hash(),
	}
	// create the full extended block with the extended data and extended header
	block := &Block{
		header: &header.ExtendedHeader{
			DAH: &dah,
		},
		data: extendedData,
	}
	// validate the encoding of the block against the data hash in the raw block header
	err = validateEncoding(block, rawBlock.Header)
	require.NoError(t, err)
}
