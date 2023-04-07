package eds

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/filecoin-project/dagstore/mount"

	"github.com/celestiaorg/celestia-node/share"
)

type RSMount struct {
	Path     string
	Datahash share.DataHash
}

func (r *RSMount) Fetch(ctx context.Context) (mount.Reader, error) {
	fReader, err := os.Open(r.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", r.Path, err)
	}
	odsReader, err := ODSReader(fReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create ODS reader: %w", err)
	}
	eds, _ := ReadEDS(ctx, odsReader, r.Datahash)
	if err != nil {
		return nil, fmt.Errorf("failed to read EDS: %w", err)
	}
	pipeReader, pipeWriter := io.Pipe()

	go func() {
		defer pipeWriter.Close()

		err := WriteEDS(ctx, eds, pipeWriter)
		if err != nil {
			log.Errorw("failed to write EDS: ", err)
		}
	}()

	carBytes, err := io.ReadAll(pipeReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read EDS from pipe: %w", err)
	}
	return bytesReader{bytes.NewReader(carBytes)}, nil
}

type bytesReader struct {
	*bytes.Reader
}

func (f *RSMount) Stat(_ context.Context) (mount.Stat, error) {
	stat, err := os.Stat(f.Path)
	if err != nil && os.IsNotExist(err) {
		return mount.Stat{}, err
	}
	return mount.Stat{
		Exists: !os.IsNotExist(err),
		Size:   stat.Size(),
	}, err
}

func (b bytesReader) Close() error {
	return nil
}

func (f *RSMount) Info() mount.Info {
	return mount.Info{
		Kind:             mount.KindLocal,
		AccessRandom:     true,
		AccessSeek:       true,
		AccessSequential: true,
	}
}

func (f *RSMount) Serialize() *url.URL {
	return &url.URL{
		Host: f.Path,
	}
}

func (f *RSMount) Deserialize(u *url.URL) error {
	if u.Host == "" {
		return fmt.Errorf("invalid host")
	}
	f.Path = u.Host
	return nil
}

func (f *RSMount) Close() error {
	return nil
}
