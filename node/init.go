package node

import (
	"os"
	"path/filepath"

	"github.com/celestiaorg/celestia-node/core"
	"github.com/celestiaorg/celestia-node/libs/fslock"
	"github.com/celestiaorg/celestia-node/libs/utils"
)

// Init initializes the Node FileSystem Repository for the given Node Type 'tp' in the directory under 'path' with
// default Config.
func Init(path string, tp Type) error {
	return InitWith(path, tp, DefaultConfig(tp))
}

// InitWith initializes the Node FileSystem Repository for the given Node Type 'tp' in the directory under 'path'
// with the given Config 'cfg'.
func InitWith(path string, tp Type, cfg *Config) error {
	path, err := repoPath(path)
	if err != nil {
		return err
	}
	log.Infof("Initializing %s Node Repository over '%s'", tp, path)

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
	defer flock.Unlock() //nolint: errcheck

	err = initDir(keysPath(path))
	if err != nil {
		return err
	}

	err = initDir(dataPath(path))
	if err != nil {
		return err
	}

	cfgPath := configPath(path)
	if !utils.Exists(cfgPath) {
		err = SaveConfig(cfgPath, cfg)
		if err != nil {
			return err
		}
		log.Infow("Saving config", "path", cfgPath)
	} else {
		log.Infow("Config already exists", "path", cfgPath)
	}

	// TODO(@Wondertan): This is a lazy hack which prevents Core Repository to be generated for all case, and generates
	//  only for a Full Node with embedded Core Node. Ideally, we should a have global map Node Type/Mode -> Custom
	//  Init Func, so Init would run initialization for specific Mode/Type.
	if !cfg.Core.Remote && tp == Full {
		corePath := corePath(path)
		err = initDir(corePath)
		if err != nil {
			return err
		}

		return core.Init(corePath)
	}

	log.Info("Node Repository initialized")
	return nil
}

// IsInit checks whether FileSystem Repository was setup under given 'path'.
// If any required file/subdirectory does not exist, then false is reported.
func IsInit(path string, tp Type) bool {
	path, err := repoPath(path)
	if err != nil {
		return false
	}

	cfg, err := LoadConfig(configPath(path)) // load the Config and implicitly check for its existence
	if err != nil {
		return false
	}

	// TODO(@Wondertan): this is undesirable hack related to the TODO above.
	//  They should be resolved together.
	if !cfg.Core.Remote && tp == Full && !core.IsInit(corePath(path)) {
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
		return nil
	}
	return os.Mkdir(path, perms)
}
