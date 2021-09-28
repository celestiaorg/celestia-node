package node

import (
	"os"
	"path/filepath"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/libs/fslock"
	"github.com/celestiaorg/celestia-node/libs/utils"
)

// Init initializes the Node FileSystem(FS) Repository in the directory under 'path' with the given Config.
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

	cfgPath := configPath(path)
	if !utils.Exists(cfgPath) {
		err = SaveConfig(cfgPath, cfg)
		if err != nil {
			return err
		}
	}

	corePath := corePath(path)
	err = initSub(corePath)
	if err != nil {
		return err
	}

	// TODO(@Wondertan): This is a lazy hack which causes Core Repository to be generated for all case,
	//  even when its not needed. Ideally, we should a have global map Node Type/Mode -> Custom Init Func, so Init would
	//  run initialization for specific Mode/Type, but there are some caveats with such approach.
	return core.Init(corePath)
}

// IsInit checks whether FS Repository was setup under given 'path'.
// If any required file/subdirectory does not exist, then false is reported.
func IsInit(path string) bool {
	path, err := repoPath(path)
	if err != nil {
		return false
	}

	if utils.Exists(corePath(path)) &&
		utils.Exists(keysPath(path)) &&
		utils.Exists(dataPath(path)) &&
		utils.Exists(configPath(path)) {
		return true
	}

	return false
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
