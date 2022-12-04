package main

import (
	"fmt"
	"path/filepath"

	"github.com/cristalhq/jwt"
	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/celestiaorg/celestia-node/api/rpc/perms"
	cmdnode "github.com/celestiaorg/celestia-node/cmd"
	"github.com/celestiaorg/celestia-node/libs/keystore"
	nodemod "github.com/celestiaorg/celestia-node/nodebuilder/node"
)

func authCmd(fsets ...*flag.FlagSet) *cobra.Command {
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

	expanded, err := homedir.Expand(filepath.Clean(cmdnode.StorePath(cmd.Context())))
	if err != nil {
		return err
	}
	ks, err := keystore.NewFSKeystore(expanded + "/keys")
	if err != nil {
		return err
	}

	key, err := ks.Get(nodemod.SecretName)
	if err != nil {
		return err
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
