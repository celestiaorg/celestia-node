package node

import (
	"os"
	"path/filepath"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/libs/fslock"
	"github.com/celestiaorg/celestia-node/libs/utils"
)

// Init initializes the Node FS Repository in the dir under 'path' with given Config 'cfg'.
func Init(path string, cfg *Config) error {
	path, err := repoPath(path)
	if err != nil {
		return err
	}

	flock, err := fslock.Lock(lockPath(path))
	if err != nil {
		if err == fslock.ErrLocked {
			return ErrOpened
		}
		return err
	}
	defer flock.Unlock() //nolint: errcheck

	err = initRoot(path)
	if err != nil {
		return err
	}

	err = initSub(keysPath(path))
	if err != nil {
		return err
	}

	err = initSub(dataPath(path))
	if err != nil {
		return err
	}

	err = initSub(corePath(path))
	if err != nil {
		return err
	}

	cfgPath := configPath(path)
	if !utils.Exists(cfgPath) {
		return SaveConfig(cfgPath, cfg)
	}

	return nil
}

// IsInit checks whether FS Repository was setup under given 'path'.
// If any required file/subdirectory does not exist, then false is reported.
func IsInit(path string) (bool, error) {
	path, err := repoPath(path)
	if err != nil {
		return false, err
	}

	if utils.Exists(corePath(path)) &&
		utils.Exists(keysPath(path)) &&
		utils.Exists(dataPath(path)) &&
		utils.Exists(configPath(path)) {
		return true, nil
	}

	return false, nil
}

const perms = 0755

func initRoot(path string) error {
	if !utils.Exists(path) {
		err := os.MkdirAll(path, perms)
		if err != nil {
			return err
		}
	}

	// check for writing permissions
	f, err := os.Create(filepath.Join(path, ".check"))
	if err != nil {
		return err
	}

	err = f.Close()
	if err != nil {
		return err
	}

	return os.Remove(f.Name())
}

func initSub(path string) error {
	if !utils.Exists(path) {
		return os.Mkdir(path, perms)
	}
	return nil
}
