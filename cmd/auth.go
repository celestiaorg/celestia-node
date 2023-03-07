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
	"github.com/celestiaorg/celestia-node/libs/keystore"
	nodemod "github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func AuthCmd(fsets ...*flag.FlagSet) *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "auth [permission-level (e.g. read || write || admin)]",
		Short: "Signs and outputs a hex-encoded JWT token with the given permissions.",
		Long: "Signs and outputs a hex-encoded JWT token with the given permissions. NOTE: only use this command when " +
			"the node has already been initialized and started.",
		RunE: newToken,
	}

	for _, set := range fsets {
		cmd.Flags().AddFlagSet(set)
	}
	return cmd
}

func newToken(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("must specify permissions")
	}

	permissions, err := convertToPerms(args[0])
	if err != nil {
		return err
	}

	expanded, err := homedir.Expand(filepath.Clean(StorePath(cmd.Context())))
	if err != nil {
		return err
	}
	ks, err := keystore.NewFSKeystore(filepath.Join(expanded, "keys"), nil)
	if err != nil {
		return err
	}

	var key keystore.PrivKey
	key, err = ks.Get(nodemod.SecretName)
	if err != nil {
		if !errors.Is(err, keystore.ErrNotFound) {
			return err
		}
		// otherwise, generate and save new priv key
		key, err = generateNewKey(ks)
		if err != nil {
			return err
		}
	}

	signer, err := jwt.NewHS256(key.Body)
	if err != nil {
		return err
	}

	token, err := jwt.NewTokenBuilder(signer).Build(&perms.JWTPayload{
		Allow: permissions,
	})
	if err != nil {
		return err
	}

	fmt.Printf("%s", token.InsecureString())
	return nil
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
