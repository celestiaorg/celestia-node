package keystore

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/multiformats/go-base32"
)

type (
	// KeyName represents private key name.
	KeyName string

	// PrivKey represents private key with arbitrary body.
	PrivKey struct {
		Body []byte `json:"body"`

		// TODO(@Wondertan): At later point, it might make sense to have a Type.
	}
)

// Keystore is meant to manage private keys.
type Keystore interface {
	// Put stores given PrivKey.
	Put(KeyName, PrivKey) error

	// Get reads PrivKey using given KeyName.
	Get(KeyName) (PrivKey, error)

	// Delete erases PrivKey using given KeyName.
	Delete(name KeyName) error

	// List lists all stored key names.
	List() ([]KeyName, error)

	// Path reports the path of the Keystore.
	Path() string

	// Keyring returns the keyring corresponding to the node's
	// keystore.
	Keyring() keyring.Keyring
}

// KeyNameFromBase32 decodes KeyName from Base32 format.
func KeyNameFromBase32(bs string) (KeyName, error) {
	name, err := base32.RawStdEncoding.DecodeString(bs)
	if err != nil {
		return "", fmt.Errorf("keystore: can't convert base32 string to key name: %w", err)
	}

	return KeyName(name), nil
}

// Base32 formats KeyName to Base32 format.
func (kn KeyName) Base32() string {
	return base32.RawStdEncoding.EncodeToString([]byte(kn))
}

func (kn KeyName) String() string {
	return string(kn)
}
