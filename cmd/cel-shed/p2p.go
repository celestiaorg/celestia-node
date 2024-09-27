package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/celestiaorg/celestia-node/nodebuilder"
	"github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/spf13/cobra"
	"go.uber.org/fx"
)

func init() {
	p2pCmd.AddCommand(p2pNewKeyCmd, p2pPeerIDCmd, p2pConnectBootstrappersCmd)
}

var p2pCmd = &cobra.Command{
	Use:   "p2p [subcommand]",
	Short: "Collection of p2p related utilities",
}

var p2pNewKeyCmd = &cobra.Command{
	Use:   "new-key",
	Short: "Generate and print new Ed25519 private key for p2p networking",
	RunE: func(_ *cobra.Command, _ []string) error {
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
	RunE: func(_ *cobra.Command, args []string) error {
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

var (
	errorOnAnyFailure bool
	errorOnAllFailure bool
)

var p2pConnectBootstrappersCmd = &cobra.Command{
	Use:   "connect-bootstrappers [network]",
	Short: "Connect to bootstrappers of a certain network",
	RunE: func(cmd *cobra.Command, args []string) error {
		network := p2p.GetNetwork(args[0])
		bootstrappers, err := p2p.BootstrappersFor(network)
		if err != nil {
			return fmt.Errorf("failed to get bootstrappers: %w", err)
		}

		store := nodebuilder.NewMemStore()
		cfg := p2p.DefaultConfig(node.Light)
		modp2p := p2p.ConstructModule(node.Light, &cfg)

		var mod p2p.Module
		app := fx.New(
			fx.NopLogger,
			modp2p,
			fx.Provide(fraud.Unmarshaler),
			fx.Provide(cmd.Context),
			fx.Provide(store.Keystore),
			fx.Provide(store.Datastore),
			fx.Supply(bootstrappers),
			fx.Supply(network),
			fx.Supply(node.Light),
			fx.Invoke(func(modprov p2p.Module) {
				mod = modprov
			}),
		)

		if err := app.Start(cmd.Context()); err != nil {
			return fmt.Errorf("failed to start app: %w", err)
		}
		defer func() {
			if err := app.Stop(cmd.Context()); err != nil {
				fmt.Printf("failed to stop application: %v\n", err)
			}
		}()

		p2pInfo, err := mod.Info(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to get p2p info: %w", err)
		}

		fmt.Printf("PeerID: %s\n", p2pInfo.ID)
		for _, addr := range p2pInfo.Addrs {
			fmt.Printf("Listening on: %s\n", addr.String())
		}
		fmt.Println()

		successfulConnections := 0
		failedConnections := 0
		for _, bootstrapper := range bootstrappers {
			fmt.Printf("trying to connect to bootstrapper: %s\n", bootstrapper)
			if err := mod.Connect(cmd.Context(), bootstrapper); err != nil {
				fmt.Printf("failed to connect to bootstrapper: %v\n", err)
				failedConnections++
				continue
			}
			fmt.Println("connected")
			successfulConnections++
		}

		if failedConnections == len(bootstrappers) && errorOnAllFailure {
			fmt.Println()
			fmt.Printf("failed to connect to all bootstrappers\n")
			os.Exit(1)
			return nil
		} else if failedConnections > 0 && errorOnAnyFailure {
			fmt.Println()
			fmt.Printf("failed to connect to some bootstrappers\n")
			os.Exit(1)
			return nil
		}

		return nil
	},
	Args: cobra.ExactArgs(1),
}

func init() {
	p2pConnectBootstrappersCmd.Flags().BoolVar(&errorOnAnyFailure, "err-any", false, "Return error if at least one bootstrapper is not reachable")
	p2pConnectBootstrappersCmd.Flags().BoolVar(&errorOnAllFailure, "err-all", false, "Return error if no bootstrapper is reachable")
}
