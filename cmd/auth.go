package cmd

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/cristalhq/jwt"
	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/api/rpc/perms"
	"github.com/celestiaorg/celestia-node/libs/authtoken"
	"github.com/celestiaorg/celestia-node/libs/keystore"
	nodemod "github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func AuthCmd(fsets ...*flag.FlagSet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth [permission-level (e.g. read || write || admin)]",
		Short: "Signs and outputs a hex-encoded JWT token with the given permissions.",
		Long: "Signs and outputs a hex-encoded JWT token with the given permissions. NOTE: only use this command when " +
			"the node has already been initialized and started.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("must specify permissions")
			}
			permissions, err := convertToPerms(args[0])
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

			token, err := buildJWTToken(key.Body, permissions)
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
	return cmd
}

func newKeystore(path string) (keystore.Keystore, error) {
	expanded, err := homedir.Expand(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	return keystore.NewFSKeystore(filepath.Join(expanded, "keys"), nil)
}

func buildJWTToken(body []byte, permissions []auth.Permission) (string, error) {
	signer, err := jwt.NewHS256(body)
	if err != nil {
		return "", err
	}
	return authtoken.NewSignedJWT(signer, permissions)
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
