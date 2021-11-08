package testutils

import (
	"bytes"
	"math"
	"math/rand"
	"sort"
	"time"

	tmbytes "github.com/celestiaorg/celestia-core/libs/bytes"
	"github.com/celestiaorg/celestia-core/pkg/consts"
	"github.com/celestiaorg/celestia-core/types"
)

// GenerateRandomBlockData returns randomly generated block data for testing purposes.
func GenerateRandomBlockData(txCount, isrCount, evdCount, msgCount, maxSize int) (types.Data, error) {
	var out types.Data
	// generate random txs
	txs, err := GenerateRandomlySizedContiguousShares(txCount, maxSize)
	if err != nil {
		return types.Data{}, err
	}
	out.Txs = txs
	// generate random intermediate state roots
	isr, err := GenerateRandomISR(isrCount)
	if err != nil {
		return types.Data{}, err
	}
	out.IntermediateStateRoots = isr
	// generate random evidence
	out.Evidence = GenerateIdenticalEvidence(evdCount)
	// generate random messages
	out.Messages = GenerateRandomlySizedMessages(msgCount, maxSize)

	return out, nil
}

// GenerateRandomlySizedContiguousShares returns a given amount of randomly
// sized (up to the given maximum size) transactions that can be included in
// dummy block data.
func GenerateRandomlySizedContiguousShares(count, max int) (types.Txs, error) {
	txs := make(types.Txs, count)
	for i := 0; i < count; i++ {
		//nolint
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
		//nolint
		_, err := rand.Read(tx)
		if err != nil {
			return nil, err
		}
		txs[i] = tx
	}
	return txs, nil
}

// GenerateRandomISR returns a given amount of randomly generated intermediate
// state roots that can be included in dummy block data.
func GenerateRandomISR(count int) (types.IntermediateStateRoots, error) {
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

// GenerateIdenticalEvidence returns a given amount of vote evidence data that
// can be included in dummy block data.
func GenerateIdenticalEvidence(count int) types.EvidenceData {
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
		//nolint
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
	shares := GenerateRandNamespacedRawData(uint32(count), consts.NamespaceSize, uint32(msgSize))
	msgs := make([]types.Message, count)
	for i, s := range shares {
		msgs[i] = types.Message{
			Data:        s[consts.NamespaceSize:],
			NamespaceID: s[:consts.NamespaceSize],
		}
	}
	return types.Messages{MessagesList: msgs}.SplitIntoShares()
}

// GenerateRandNamespacedRawData returns random namespaced raw data for testing purposes.
func GenerateRandNamespacedRawData(total, nidSize, leafSize uint32) [][]byte {
	data := make([][]byte, total)
	for i := uint32(0); i < total; i++ {
		nid := make([]byte, nidSize)
		//nolint
		rand.Read(nid)
		data[i] = nid
	}
	sortByteArrays(data)
	for i := uint32(0); i < total; i++ {
		d := make([]byte, leafSize)
		//nolint
		rand.Read(d)
		data[i] = append(data[i], d...)
	}

	return data
}

func sortByteArrays(src [][]byte) {
	sort.Slice(src, func(i, j int) bool { return bytes.Compare(src[i], src[j]) < 0 })
}
