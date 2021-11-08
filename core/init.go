package core

import (
	"fmt"
	"path/filepath"

	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

// defaultValKeyType is a default crypto algo for consensus in core
const defaultValKeyType = types.ABCIPubKeyTypeSecp256k1

// Init checks each item of the list(below) for existence in the path.
// If latter(item) is not existing, then it(func Init) creates them by default settings
// List of items:
// * Config
// * Private validator key
// * P2P comms private key
// * Stub genesis doc file
func Init(path string) (err error) {
	log.Infof("Initializing Core Repository over '%s'", path)
	defer log.Info("Core Repository initialized")

	// 1 - ensure config
	cfg := DefaultConfig()
	cfg.SetRoot(path)
	cfgPath := configPath(path)
	if !utils.Exists(cfgPath) {
		err = SaveConfig(cfgPath, cfg)
		if err != nil {
			return fmt.Errorf("core: can't write config: %w", err)
		}
		log.Info("New config is generated")
	} else {
		log.Info("Config already exists")
	}
	// 2 - ensure private validator key
	var pv *privval.FilePV
	keyPath := cfg.PrivValidatorKeyFile()
	if !utils.Exists(keyPath) {
		pv, err = privval.GenFilePV(keyPath, cfg.PrivValidatorStateFile(), defaultValKeyType)
		if err != nil {
			return
		}
		pv.Save()
		log.Info("New consensus private key is generated")
	} else {
		pv = privval.LoadFilePV(keyPath, cfg.PrivValidatorStateFile())
		log.Info("Consensus private key already exists")
	}
	// 3 - ensure private p2p key
	_, err = p2p.LoadOrGenNodeKey(cfg.NodeKeyFile())
	if err != nil {
		return
	}
	// 4 - ensure genesis
	genPath := cfg.GenesisFile()
	if !utils.Exists(genPath) {
		log.Info("New stub genesis document is generated")
		log.Warn("Stub genesis document must not be used in production environment!")
		pubKey, err := pv.GetPubKey()
		if err != nil {
			return fmt.Errorf("can't get pubkey: %w", err)
		}

		params := types.DefaultConsensusParams()
		params.Validator.PubKeyTypes = []string{defaultValKeyType}
		genDoc := types.GenesisDoc{
			ChainID:         fmt.Sprintf("localnet-%v", tmrand.Str(6)),
			GenesisTime:     tmtime.Now(),
			ConsensusParams: params,
			Validators: []types.GenesisValidator{{
				Address: pubKey.Address(),
				PubKey:  pubKey,
				Power:   10,
			}},
		}

		err = genDoc.SaveAs(genPath)
		if err != nil {
			return err
		}
	} else {
		log.Info("Genesis document already exists")
	}

	return nil
}

// IsInit checks whether the Core is initialized under the given 'path'.
// It requires Config, Private Validator Key, Private P2P Key and genesis doc.
func IsInit(path string) bool {
	cfgPath := configPath(path)
	if !utils.Exists(cfgPath) {
		return false
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		log.Errorf("loading config: %s", err)
		return false
	}

	if !utils.Exists(cfg.PrivValidatorKeyFile()) ||
		!utils.Exists(cfg.NodeKeyFile()) ||
		!utils.Exists(cfg.GenesisFile()) {
		return false
	}

	return true
}

func configPath(base string) string {
	return filepath.Join(base, "config.toml")
}
