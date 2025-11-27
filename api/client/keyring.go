package client

import (
	"errors"
	"fmt"
	"os"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/celestiaorg/celestia-app/v6/app"
	"github.com/celestiaorg/celestia-app/v6/app/encoding"
)

// KeyringConfig contains configuration parameters for constructing
// the node's keyring signer.
type KeyringConfig struct {
	KeyName     string
	BackendName string
}

// KeyringWithNewKey is a helper function for easy keyring initialization. It initializes a keyring
// with the given config and path. It creates a new key if no key is found in the keyring store.
func KeyringWithNewKey(cfg KeyringConfig, path string) (keyring.Keyring, error) {
	if cfg.KeyName == "" {
		return nil, errors.New("no key name provided")
	}
	encConf := encoding.MakeConfig(app.ModuleEncodingRegisters...)

	if cfg.BackendName == keyring.BackendTest {
		log.Warn("Detected plaintext keyring backend. For elevated security properties, consider using" +
			" the `file` keyring backend.")
	}
	ring, err := keyring.New(app.Name, cfg.BackendName, path, os.Stdin, encConf.Codec)
	if err != nil {
		return nil, err
	}

	// check if key already exists
	_, err = ring.Key(cfg.KeyName)
	if err == nil {
		return ring, nil
	}

	if !errors.Is(err, sdkerrors.ErrKeyNotFound) {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}
	log.Infow("NO KEY FOUND IN STORE, GENERATING NEW KEY...")
	keyInfo, mn, err := ring.NewMnemonic(cfg.KeyName, keyring.English, sdk.GetConfig().GetFullBIP44Path(),
		keyring.DefaultBIP39Passphrase, hd.Secp256k1)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	log.Info("NEW KEY GENERATED...")
	addr, err := keyInfo.GetAddress()
	if err != nil {
		return nil, fmt.Errorf("failed to get address: %w", err)
	}
	fmt.Printf("\nNAME: %s\nADDRESS: %s\nMNEMONIC (save this somewhere safe!!!): \n%s\n\n",
		keyInfo.Name, addr.String(), mn)

	return ring, nil
}
