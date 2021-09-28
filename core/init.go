package core

import (
	"fmt"
	"path/filepath"

	tmrand "github.com/celestiaorg/celestia-core/libs/rand"
	"github.com/celestiaorg/celestia-core/p2p"
	"github.com/celestiaorg/celestia-core/privval"
	"github.com/celestiaorg/celestia-core/types"
	tmtime "github.com/celestiaorg/celestia-core/types/time"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

// defaultValKeyType is default crypto algo for consensus in core
const defaultValKeyType = types.ABCIPubKeyTypeSecp256k1

// Init initializes the Core repository under the given 'path'.
// It checks and creates the default if not exist:
// * Config
// * Private validator key
// * P2P comms private key
// * Stub genesis doc file
func Init(path string) (err error) {
	log.Info("Initializing repository over '%s'", path)
	// 1 - ensure config
	cfg := DefaultConfig()
	cfg.SetRoot(path)
	cfgPath := configPath(path)
	if !utils.Exists(cfgPath) {
		err = SaveConfig(cfgPath, cfg)
		if err != nil {
			return fmt.Errorf("core: can't write config: %w", err)
		}
	}
	// 2 - ensure private validator key
	var pv *privval.FilePV
	keyPath := cfg.PrivValidatorKeyFile()
	if utils.Exists(keyPath) {
		pv = privval.LoadFilePV(keyPath, cfg.PrivValidatorStateFile())
	} else {
		pv, err = privval.GenFilePV(keyPath, cfg.PrivValidatorStateFile(), defaultValKeyType)
		if err != nil {
			return
		}
		pv.Save()
	}
	// 3 - ensure private p2p key
	_, err = p2p.LoadOrGenNodeKey(cfg.NodeKeyFile())
	if err != nil {
		return
	}
	// 4 - ensure genesis
	genPath := cfg.GenesisFile()
	if !utils.Exists(genPath) {
		log.Warn("Genesis file is not found - generating a stub")
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

		return genDoc.SaveAs(genPath)
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
