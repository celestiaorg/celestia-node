package block

import (
	"bytes"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/pkg/consts"
	"github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-node/service/header"
)

func Test_validateEncoding(t *testing.T) {
	// generate random raw block data
	rawData, err := GenerateRandomBlockData(1, 1, 1, 1, 40)
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

// GenerateRandomBlockData returns randomly generated block data for testing purposes.
func GenerateRandomBlockData(txCount, isrCount, evdCount, msgCount, maxSize int) (types.Data, error) {
	var out types.Data
	// generate random txs
	txs, err := generateRandomlySizedContiguousShares(txCount, maxSize)
	if err != nil {
		return types.Data{}, err
	}
	out.Txs = txs
	// generate random intermediate state roots
	isr, err := generateRandomISR(isrCount)
	if err != nil {
		return types.Data{}, err
	}
	out.IntermediateStateRoots = isr
	// generate random evidence
	out.Evidence = generateIdenticalEvidence(evdCount)
	// generate random messages
	out.Messages = GenerateRandomlySizedMessages(msgCount, maxSize)

	return out, nil
}

// generateRandomlySizedContiguousShares returns a given amount of randomly
// sized (up to the given maximum size) transactions that can be included in
// dummy block data.
func generateRandomlySizedContiguousShares(count, max int) (types.Txs, error) {
	txs := make(types.Txs, count)
	for i := 0; i < count; i++ {
		size := rand.Intn(max)
		// ensure that no transactions are 0 bytes, as no valid transaction has only 0 bytes
		if size == 0 {
			size = 1
		}
		tx, err := generateRandomContiguousShares(1, size)
		if err != nil {
			return nil, err
		}
		txs[i] = tx[0]
	}
	return txs, nil
}

func generateRandomContiguousShares(count, size int) (types.Txs, error) {
	txs := make(types.Txs, count)
	for i := 0; i < count; i++ {
		tx := make([]byte, size)

		_, err := rand.Read(tx)
		if err != nil {
			return nil, err
		}
		txs[i] = tx
	}
	return txs, nil
}

// generateRandomISR returns a given amount of randomly generated intermediate
// state roots that can be included in dummy block data.
func generateRandomISR(count int) (types.IntermediateStateRoots, error) {
	roots := make([]tmbytes.HexBytes, count)
	for i := 0; i < count; i++ {
		shares, err := generateRandomContiguousShares(1, 32)
		if err != nil {
			return types.IntermediateStateRoots{}, err
		}
		roots[i] = tmbytes.HexBytes(shares[0])
	}
	return types.IntermediateStateRoots{RawRootsList: roots}, nil
}

// generateIdenticalEvidence returns a given amount of vote evidence data that
// can be included in dummy block data.
func generateIdenticalEvidence(count int) types.EvidenceData {
	evidence := make([]types.Evidence, count)
	for i := 0; i < count; i++ {
		ev := types.NewMockDuplicateVoteEvidence(math.MaxInt64, time.Now(), "chainID")
		evidence[i] = ev
	}
	return types.EvidenceData{Evidence: evidence}
}

// GenerateRandomlySizedMessages returns a given amount of Messages up to the given maximum
// message size that can be included in dummy block data.
func GenerateRandomlySizedMessages(count, maxMsgSize int) types.Messages {
	msgs := make([]types.Message, count)
	for i := 0; i < count; i++ {

		msgs[i] = generateRandomMessage(rand.Intn(maxMsgSize))
	}

	// this is just to let us use assert.Equal
	if count == 0 {
		msgs = nil
	}

	return types.Messages{MessagesList: msgs}
}

func generateRandomMessage(size int) types.Message {
	share := generateRandomNamespacedShares(1, size)[0]
	msg := types.Message{
		NamespaceID: share.NamespaceID(),
		Data:        share.Data(),
	}
	return msg
}

func generateRandomNamespacedShares(count, msgSize int) types.NamespacedShares {
	shares := generateRandNamespacedRawData(uint32(count), consts.NamespaceSize, uint32(msgSize))
	msgs := make([]types.Message, count)
	for i, s := range shares {
		msgs[i] = types.Message{
			Data:        s[consts.NamespaceSize:],
			NamespaceID: s[:consts.NamespaceSize],
		}
	}
	return types.Messages{MessagesList: msgs}.SplitIntoShares()
}

// generateRandNamespacedRawData returns random namespaced raw data for testing purposes.
func generateRandNamespacedRawData(total, nidSize, leafSize uint32) [][]byte {
	data := make([][]byte, total)
	for i := uint32(0); i < total; i++ {
		nid := make([]byte, nidSize)

		rand.Read(nid)
		data[i] = nid
	}
	sortByteArrays(data)
	for i := uint32(0); i < total; i++ {
		d := make([]byte, leafSize)

		rand.Read(d)
		data[i] = append(data[i], d...)
	}

	return data
}

func sortByteArrays(src [][]byte) {
	sort.Slice(src, func(i, j int) bool { return bytes.Compare(src[i], src[j]) < 0 })
}
