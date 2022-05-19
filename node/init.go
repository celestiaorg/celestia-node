package node

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/celestiaorg/celestia-node/libs/fslock"
	"github.com/celestiaorg/celestia-node/libs/utils"
	"github.com/celestiaorg/celestia-node/params"
)

// Init initializes the Node FileSystem Store for the given Node Type 'tp' in the directory under 'path' with
// default Config. Options are applied over default Config and persisted on disk.
func Init(path string, tp Type, options ...Option) error {
	sets := &settings{cfg: DefaultConfig(tp)}
	for _, option := range options {
		option(sets)
	}

	path, err := storePath(path)
	if err != nil {
		return err
	}
	log.Infof("Initializing %s Node Store over '%s'", tp, path)

	err = initRoot(path)
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
	defer flock.Unlock() // nolint: errcheck

	err = initDir(keysPath(path))
	if err != nil {
		return err
	}

	err = initDir(dataPath(path))
	if err != nil {
		return err
	}

	if sets.cfg == nil {
		return errors.New("configuration is missing for the node's initialisation")
	}

	cfgPath := configPath(path)
	if !utils.Exists(cfgPath) {
		err = SaveConfig(cfgPath, sets.cfg)
		if err != nil {
			return err
		}
		log.Infow("Saving config", "path", cfgPath)
	} else {
		log.Infow("Config already exists", "path", cfgPath)
	}

	log.Info("Node Store initialized")
	return nil
}

// IsInit checks whether FileSystem Store was setup under given 'path'.
// If any required file/subdirectory does not exist, then false is reported.
func IsInit(path string) bool {
	path, err := storePath(path)
	if err != nil {
		log.Errorw("parsing store path", "path", path, "err", err)
		return false
	}

	_, err = LoadConfig(configPath(path)) // load the Config and implicitly check for its existence
	if err != nil {
		log.Errorw("loading config", "path", path, "err", err)
		return false
	}

	if utils.Exists(keysPath(path)) &&
		utils.Exists(dataPath(path)) {
		return true
	}

	return false
}

const perms = 0755

// initRoot initializes(creates) directory if not created and check if it is writable
func initRoot(path string) error {
	err := initDir(path)
	if err != nil {
		return err
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

// initDir creates a dir if not exist
func initDir(path string) error {
	if utils.Exists(path) {
		// if the dir already exists and `CELESTIA_CUSTOM` env var is set,
		// fail out to prevent store corruption
		if _, ok := os.LookupEnv(params.EnvCustomNetwork); ok {
			return fmt.Errorf("cannot run a custom network over an already-existing node store")
		}
		return nil
	}
	return os.Mkdir(path, perms)
}
