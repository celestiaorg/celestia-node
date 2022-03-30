package main

import (
	"github.com/spf13/cobra"

	cmdnode "github.com/celestiaorg/celestia-node/cmd"
)

func parseFlags(cmd *cobra.Command, args []string) error {
	env, err := cmdnode.GetEnv(cmd.Context())
	if err != nil {
		return err
	}

	err = cmdnode.ParseNodeFlags(cmd, env)
	if err != nil {
		return err
	}

	err = cmdnode.ParseP2PFlags(cmd, env)
	if err != nil {
		return err
	}

	err = cmdnode.ParseCoreFlags(cmd, env)
	if err != nil {
		return err
	}

	err = cmdnode.ParseTrustedHashFlags(cmd, env)
	if err != nil {
		return err
	}

	err = cmdnode.ParseMiscFlags(cmd)
	if err != nil {
		return err
	}

	return nil
}
