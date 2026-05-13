package headers

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	dsbadger "github.com/ipfs/go-ds-badger4"

	libhead_store "github.com/celestiaorg/go-header/store"

	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/header/headertest"
)

func TestHeadersPersistAfterStopAndHeadWrite(t *testing.T) {
	const totalHeaders = 2048

	dir := t.TempDir()
	ds, err := dsbadger.NewDatastore(filepath.Join(dir, "data"), nil)
	if err != nil {
		t.Fatalf("open badger: %v", err)
	}

	hstore, err := libhead_store.NewStore[*header.ExtendedHeader](
		ds,
		libhead_store.WithWriteBatchSize(1024),
	)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if err := hstore.Start(context.Background()); err != nil {
		t.Fatalf("start store: %v", err)
	}

	headers := headertest.NewTestSuite(t, 3, 0).GenExtendedHeaders(totalHeaders)
	for i := 0; i < len(headers); i += 256 {
		end := i + 256
		if end > len(headers) {
			end = len(headers)
		}
		if err := hstore.Append(context.Background(), headers[i:end]...); err != nil {
			t.Fatalf("append [%d:%d]: %v", i, end, err)
		}
	}
	stopHeaderStore(t, hstore)

	last := headers[len(headers)-1]
	if err := PersistHeadKey(ds, last); err != nil {
		t.Fatalf("persist head: %v", err)
	}
	if err := ds.Close(); err != nil {
		t.Fatalf("close badger: %v", err)
	}

	ds, err = dsbadger.NewDatastore(filepath.Join(dir, "data"), nil)
	if err != nil {
		t.Fatalf("reopen badger: %v", err)
	}
	defer ds.Close()

	hstore, err = libhead_store.NewStore[*header.ExtendedHeader](
		ds,
		libhead_store.WithWriteBatchSize(1024),
	)
	if err != nil {
		t.Fatalf("new reopened store: %v", err)
	}
	if err := hstore.Start(context.Background()); err != nil {
		t.Fatalf("start reopened store: %v", err)
	}
	defer stopHeaderStore(t, hstore)

	head, err := hstore.Head(context.Background())
	if err != nil {
		t.Fatalf("read head: %v", err)
	}
	if head.Height() != last.Height() || !bytes.Equal(head.Hash(), last.Hash()) {
		t.Fatalf("head = height %d hash %s, want height %d hash %s",
			head.Height(), head.Hash(), last.Height(), last.Hash())
	}

	for _, want := range headers {
		got, err := hstore.GetByHeight(context.Background(), want.Height())
		if err != nil {
			t.Fatalf("get height %d: %v", want.Height(), err)
		}
		if !bytes.Equal(got.Hash(), want.Hash()) {
			t.Fatalf("height %d hash = %s, want %s", want.Height(), got.Hash(), want.Hash())
		}
	}
}

func TestPersistHeadKeyRequiresCommittedHeight(t *testing.T) {
	dir := t.TempDir()
	ds, err := dsbadger.NewDatastore(filepath.Join(dir, "data"), nil)
	if err != nil {
		t.Fatalf("open badger: %v", err)
	}
	defer ds.Close()

	h := headertest.NewTestSuite(t, 3, 0).GenExtendedHeaders(1)[0]
	if err := PersistHeadKey(ds, h); err == nil {
		t.Fatal("PersistHeadKey succeeded for a header that was not committed")
	}
}

func stopHeaderStore(t *testing.T, hstore *libhead_store.Store[*header.ExtendedHeader]) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := hstore.Stop(ctx); err != nil {
		t.Fatalf("stop store: %v", err)
	}
}
