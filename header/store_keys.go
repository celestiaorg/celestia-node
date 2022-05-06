package header

import (
	"strconv"

	"github.com/ipfs/go-datastore"
)

var (
	storePrefix = datastore.NewKey("headers")
	headKey     = datastore.NewKey("head")
)

func heightKey(h uint64) datastore.Key {
	return datastore.NewKey(strconv.Itoa(int(h)))
}

func headerKey(h *ExtendedHeader) datastore.Key {
	return datastore.NewKey(h.Hash().String())
}
