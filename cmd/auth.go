package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/cristalhq/jwt/v5"
	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/gofrs/flock"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/api/rpc/perms"
	"github.com/celestiaorg/celestia-node/libs/authtoken"
	"github.com/celestiaorg/celestia-node/libs/keystore"
	nodemod "github.com/celestiaorg/celestia-node/nodebuilder/node"
)

var ttlFlagName = "ttl"

func AuthCmd(fsets ...*flag.FlagSet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth [permission-level (e.g. read || write || admin)]",
		Short: "Signs and outputs a hex-encoded JWT token with the given permissions.",
		Long: "Signs and outputs a hex-encoded JWT token with the given permissions. NOTE: only use this command when " +
			"the node has already been initialized and started.",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := ParseStoreDeterminationFlags(cmd, NodeType(cmd.Context()), args)
			if err != nil {
				return err
			}

			if len(args) != 1 {
				return errors.New("must specify permissions")
			}
			permissions, err := convertToPerms(args[0])
			if err != nil {
				return err
			}

			ttl, err := cmd.Flags().GetDuration(ttlFlagName)
			if err != nil {
				return err
			}

			ks, err := newKeystore(StorePath(cmd.Context()))
			if err != nil {
				return err
			}

			key, err := ks.Get(nodemod.SecretName)
			if err != nil {
				if !errors.Is(err, keystore.ErrNotFound) {
					return err
				}
				key, err = generateNewKey(ks)
				if err != nil {
					return err
				}
			}

			token, err := buildJWTToken(key.Body, permissions, ttl)
			if err != nil {
				return err
			}
			fmt.Printf("%s\n", token)
			return nil
		},
	}

	for _, set := range fsets {
		cmd.Flags().AddFlagSet(set)
	}
	cmd.Flags().Duration(ttlFlagName, 0, "Set a Time-to-live (TTL) for the token")

	cmd.AddCommand(authRevokeCmd(fsets...), authRevokedCmd(fsets...))
	return cmd
}

func authRevokeCmd(fsets ...*flag.FlagSet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revoke [token-or-hex-nonce]",
		Short: "Revoke a previously-issued JWT.",
		Long: "Adds the token's nonce to the on-disk revocation set. Errors if the node is running; " +
			"use the node.AuthRevoke RPC in that case.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ParseStoreDeterminationFlags(cmd, NodeType(cmd.Context()), args); err != nil {
				return err
			}
			if len(args) != 1 {
				return errors.New("must specify a token or hex-encoded nonce")
			}
			nonce, err := resolveNonce(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			revoker, unlock, err := openRevoker(cmd.Context())
			if err != nil {
				return err
			}
			defer unlock()
			if err := revoker.Revoke(nonce); err != nil {
				return err
			}
			fmt.Printf("revoked %s\n", hex.EncodeToString(nonce))
			return nil
		},
	}
	for _, set := range fsets {
		cmd.Flags().AddFlagSet(set)
	}
	return cmd
}

func authRevokedCmd(fsets ...*flag.FlagSet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revoked",
		Short: "List hex-encoded nonces of revoked JWTs.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ParseStoreDeterminationFlags(cmd, NodeType(cmd.Context()), args); err != nil {
				return err
			}
			revoker, unlock, err := openRevoker(cmd.Context())
			if err != nil {
				return err
			}
			defer unlock()
			for _, n := range revoker.List() {
				fmt.Println(n)
			}
			return nil
		},
	}
	for _, set := range fsets {
		cmd.Flags().AddFlagSet(set)
	}
	return cmd
}

// resolveNonce returns the nonce from a JWT (needs the node's signing key) or from a hex string.
func resolveNonce(ctx context.Context, arg string) ([]byte, error) {
	ks, err := newKeystore(StorePath(ctx))
	if err != nil {
		return nil, err
	}
	if key, err := ks.Get(nodemod.SecretName); err == nil {
		verifier, err := jwt.NewVerifierHS(jwt.HS256, key.Body)
		if err == nil {
			if p, err := authtoken.ExtractSignedPayload(verifier, arg); err == nil {
				return p.Nonce, nil
			}
		}
	}
	nonce, err := hex.DecodeString(arg)
	if err != nil {
		return nil, fmt.Errorf("argument is neither a valid JWT for this node nor a hex nonce: %w", err)
	}
	return nonce, nil
}

// openRevoker takes the store's .lock so this command fails fast when the node
// is running (avoiding a silent overwrite race with the in-memory set). The
// returned unlock releases the lock and never returns an error.
func openRevoker(ctx context.Context) (*nodemod.Revoker, func(), error) {
	expanded, err := homedir.Expand(filepath.Clean(StorePath(ctx)))
	if err != nil {
		return nil, nil, err
	}
	flk := flock.New(filepath.Join(expanded, ".lock"))
	ok, err := flk.TryLock()
	if err != nil {
		return nil, nil, fmt.Errorf("locking store: %w", err)
	}
	if !ok {
		return nil, nil, errors.New("store is in use by a running node; use the node.AuthRevoke RPC instead")
	}
	unlock := func() { _ = flk.Unlock() }
	r, err := nodemod.NewRevoker(nodemod.RevokedTokensPath(expanded))
	if err != nil {
		unlock()
		return nil, nil, err
	}
	return r, unlock, nil
}

func newKeystore(path string) (keystore.Keystore, error) {
	expanded, err := homedir.Expand(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	return keystore.NewFSKeystore(filepath.Join(expanded, "keys"), nil)
}

func buildJWTToken(body []byte, permissions []auth.Permission, ttl time.Duration) (string, error) {
	signer, err := jwt.NewSignerHS(jwt.HS256, body)
	if err != nil {
		return "", err
	}
	return authtoken.NewSignedJWT(signer, permissions, ttl)
}

func generateNewKey(ks keystore.Keystore) (keystore.PrivKey, error) {
	sk, err := io.ReadAll(io.LimitReader(rand.Reader, 32))
	if err != nil {
		return keystore.PrivKey{}, err
	}
	// save key
	key := keystore.PrivKey{Body: sk}
	err = ks.Put(nodemod.SecretName, key)
	if err != nil {
		return keystore.PrivKey{}, err
	}
	return key, nil
}

func convertToPerms(perm string) ([]auth.Permission, error) {
	perms, ok := stringsToPerms[perm]
	if !ok {
		return nil, fmt.Errorf("invalid permission specified: %s", perm)
	}
	return perms, nil
}

var stringsToPerms = map[string][]auth.Permission{
	"public": perms.DefaultPerms,
	"read":   perms.ReadPerms,
	"write":  perms.ReadWritePerms,
	"admin":  perms.AllPerms,
}
