package eds

import "github.com/celestiaorg/celestia-node/share"

type FileStore struct {
	baspath string
}

func (fs *FileStore) File(hash share.DataHash) (*File, error) {
	// TODO(@Wondertan): Caching
	return OpenFile(fs.baspath + "/" + hash.String())
}
