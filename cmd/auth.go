package cmd

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/cristalhq/jwt/v5"
	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/api/rpc/perms"
	"github.com/celestiaorg/celestia-node/libs/authtoken"
	"github.com/celestiaorg/celestia-node/libs/keystore"
	nodemod "github.com/celestiaorg/celestia-node/nodebuilder/node"
)

// Define constants for better readability and maintainability.
const (
	ttlFlagName = "ttl"
	// KeySize defines the required length for the HMAC key (32 bytes = 256 bits).
	KeySize = 32
)

// stringsToPerms maps string aliases to their corresponding permission sets.
var stringsToPerms = map[string][]auth.Permission{
	"public": perms.DefaultPerms,
	"read": 	perms.ReadPerms,
	"write":	perms.ReadWritePerms,
	"admin":	perms.AllPerms,
}

// AuthCmd returns the Cobra command for generating JWT authentication tokens.
func AuthCmd(fsets ...*flag.FlagSet) *cobra.Command {
	cmd := &cobra.Command{
		Use: 	"auth [permission-level (e.g. read | write | admin)]",
		Short: "Signs and outputs a hex-encoded JWT token with the given permissions.",
		Long: "Signs and outputs a hex-encoded JWT token with the given permissions. " +
			"NOTE: only use this command when the node has already been initialized and started.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Handle command-line arguments and flags.
			if len(args) != 1 {
				return errors.New("must specify permissions: one of read, write, or admin")
			}
			
			if err := ParseStoreDeterminationFlags(cmd, NodeType(cmd.Context()), args); err != nil {
				return err
			}

			permissions, err := convertToPerms(args[0])
			if err != nil {
				return err
			}

			ttl, err := cmd.Flags().GetDuration(ttlFlagName)
			if err != nil {
				return err
			}

			// 2. Resolve Keystore path and initialize.
			storePath := StorePath(cmd.Context())
			ks, err := newKeystore(storePath)
			if err != nil {
				return err
			}

			// 3. Delegate core logic to a separate function for easier testing.
			token, err := runAuthLogic(ks, permissions, ttl)
			if err != nil {
				return err
			}
			
			// Use Println for cleaner output.
			fmt.Println(token)
			return nil
		},
	}

	for _, set := range fsets {
		cmd.Flags().AddFlagSet(set)
	}
	// Default TTL is set to 0 (no expiration).
	cmd.Flags().Duration(ttlFlagName, 0, "Set a Time-to-live (TTL) for the token (e.g., 24h)")

	return cmd
}

// runAuthLogic handles key retrieval/generation and token building.
func runAuthLogic(ks keystore.Keystore, permissions []auth.Permission, ttl time.Duration) (string, error) {
	key, err := ks.Get(nodemod.SecretName)
	if err != nil {
		// Use direct error handling for the specific "not found" case.
		if errors.Is(err, keystore.ErrNotFound) {
			// Key not found, generate a new one.
			key, err = generateNewKey(ks)
			if err != nil {
				return "", fmt.Errorf("failed to generate new key: %w", err)
			}
		} else {
			// Return other keystore-related errors.
			return "", err
		}
	}

	token, err := buildJWTToken(key.Body, permissions, ttl)
	if err != nil {
		return "", fmt.Errorf("failed to build JWT token: %w", err)
	}
	return token, nil
}

// newKeystore expands the path and initializes the FS keystore.
func newKeystore(path string) (keystore.Keystore, error) {
	expanded, err := homedir.Expand(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("failed to expand home directory path: %w", err)
	}
	// Added 'keys' subdirectory to maintain standard keystore structure.
	return keystore.NewFSKeystore(filepath.Join(expanded, "keys"), nil)
}

// buildJWTToken creates a new HMAC signer and signs the JWT with the given claims.
func buildJWTToken(body []byte, permissions []auth.Permission, ttl time.Duration) (string, error) {
	signer, err := jwt.NewSignerHS(jwt.HS256, body)
	if err != nil {
		return "", fmt.Errorf("failed to create JWT signer: %w", err)
	}
	// Use the library function for standardized token creation.
	return authtoken.NewSignedJWT(signer, permissions, ttl)
}

// generateNewKey creates a new cryptographic random key and stores it in the keystore.
func generateNewKey(ks keystore.Keystore) (keystore.PrivKey, error) {
	// Use the defined constant KeySize for clarity.
	sk := make([]byte, KeySize)
	_, err := io.ReadFull(rand.Reader, sk)
	if err != nil {
		return keystore.PrivKey{}, fmt.Errorf("failed to read random bytes for key: %w", err)
	}
	
	// Save key
	key := keystore.PrivKey{Body: sk}
	err = ks.Put(nodemod.SecretName, key)
	if err != nil {
		return keystore.PrivKey{}, fmt.Errorf("failed to save generated key: %w", err)
	}
	return key, nil
}

// convertToPerms validates the string permission alias and returns the corresponding permission slice.
func convertToPerms(perm string) ([]auth.Permission, error) {
	perms, ok := stringsToPerms[perm]
	if !ok {
		return nil, fmt.Errorf("invalid permission specified: %s. Must be one of public, read, write, or admin", perm)
	}
	return perms, nil
}
