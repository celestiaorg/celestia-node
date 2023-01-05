package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/spf13/cobra"
)

func init() {
	p2pCmd.AddCommand(p2pNewKeyCmd, p2pPeerIDCmd)
}

var p2pCmd = &cobra.Command{
	Use:   "p2p [subcommand]",
	Short: "Collection of p2p related utilities",
}

var p2pNewKeyCmd = &cobra.Command{
	Use:   "new-key",
	Short: "Generate and print new Ed25519 private key for p2p networking",
	RunE: func(cmd *cobra.Command, args []string) error {
		privkey, _, err := crypto.GenerateEd25519Key(rand.Reader)
		if err != nil {
			return err
		}

		raw, err := privkey.Raw()
		if err != nil {
			return err
		}

		fmt.Println(hex.EncodeToString(raw))
		return nil
	},
	Args: cobra.NoArgs,
}

var p2pPeerIDCmd = &cobra.Command{
	Use:   "peer-id",
	Short: "Get peer-id out of public or private Ed25519 key",
	RunE: func(cmd *cobra.Command, args []string) error {
		decKey, err := hex.DecodeString(args[0])
		if err != nil {
			return err
		}

		privKey, err := crypto.UnmarshalEd25519PrivateKey(decKey)
		if err != nil {
			// try pubkey then
			pubKey, err := crypto.UnmarshalEd25519PublicKey(decKey)
			if err != nil {
				return err
			}

			id, err := peer.IDFromPublicKey(pubKey)
			if err != nil {
				return err
			}

			fmt.Println(id.String())
			return nil
		}

		id, err := peer.IDFromPrivateKey(privKey)
		if err != nil {
			return err
		}

		fmt.Println(id.String())
		return nil
	},
	Args: cobra.ExactArgs(1),
}
