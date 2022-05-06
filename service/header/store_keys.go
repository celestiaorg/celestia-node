package header

import (
	"strconv"

	"github.com/ipfs/go-datastore"

	extheader "github.com/celestiaorg/celestia-node/service/header/extheader"
)

var (
	storePrefix = datastore.NewKey("headers")
	headKey     = datastore.NewKey("head")
)

func heightKey(h uint64) datastore.Key {
	return datastore.NewKey(strconv.Itoa(int(h)))
}

func headerKey(h *extheader.ExtendedHeader) datastore.Key {
	return datastore.NewKey(h.Hash().String())
}
