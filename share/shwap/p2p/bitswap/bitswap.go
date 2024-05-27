package bitswap

import (
	"context"
	"fmt"
	"hash"
	"sync"

	"github.com/ipfs/boxo/exchange"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	logger "github.com/ipfs/go-log/v2"
	mh "github.com/multiformats/go-multihash"

	"github.com/celestiaorg/celestia-node/share"
)

var log = logger.Logger("shwap/bitswap")

// TODO:
//  * Synchronization for GetContainers
//  * Test with race and count 100
//  * Hasher test
//  * Coverage
//  * godoc
//    * document steps required to add new id/container type

type ID[C any] interface {
	String() string
	CID() cid.Cid
	UnmarshalContainer(*share.Root, []byte) (C, error)
}

func RegisterID(mhcode, codec uint64, size int, bldrFn func(cid2 cid.Cid) (blockBuilder, error)) {
	mh.Register(mhcode, func() hash.Hash {
		return &hasher{IDSize: size}
	})
	specRegistry[mhcode] = idSpec{
		size:    size,
		codec:   codec,
		builder: bldrFn,
	}
}

// GetContainers
// Does not guarantee synchronization. Calling this func simultaneously with the same ID may cause
// issues. TODO: Describe motivation
func GetContainers[C any](ctx context.Context, fetcher exchange.Fetcher, root *share.Root, ids ...ID[C]) ([]C, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cids := make([]cid.Cid, len(ids))
	cntrs := make([]C, len(ids))
	for i, id := range ids {
		i := i
		cids[i] = id.CID()

		idStr := id.String()
		globalVerifiers.add(idStr, func(data []byte) error {
			cntr, err := id.UnmarshalContainer(root, data)
			if err != nil {
				return err
			}

			cntrs[i] = cntr
			return nil
		})
		defer globalVerifiers.release(idStr)
	}

	// must start getting only after verifiers are registered
	blkCh, err := fetcher.GetBlocks(ctx, cids)
	if err != nil {
		return nil, fmt.Errorf("fetching bitswap blocks: %w", err)
	}

	// GetBlocks handles ctx and closes blkCh, so we don't have to
	blks := make([]blocks.Block, 0, len(cids))
	for blk := range blkCh {
		blks = append(blks, blk)
	}
	if len(blks) != len(cids) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("not all the containers were found")
	}

	return cntrs, nil
}

var globalVerifiers verifiers

type verifiers struct {
	sync.Map
}

func (vs *verifiers) add(key string, v func([]byte) error) {
	vs.Store(key, v)
}

func (vs *verifiers) get(key string) func([]byte) error {
	v, ok := vs.Load(key)
	if !ok {
		return nil
	}
	return v.(func([]byte) error)
}

func (vs *verifiers) release(key string) {
	vs.Delete(key)
}
