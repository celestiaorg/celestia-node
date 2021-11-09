module github.com/celestiaorg/celestia-node

go 1.16

replace github.com/ipfs/go-verifcid => github.com/celestiaorg/go-verifcid v0.0.1-lazypatch

require (
	github.com/BurntSushi/toml v0.4.1
	github.com/celestiaorg/celestia-app v0.0.0-00010101000000-000000000000
	github.com/celestiaorg/nmt v0.7.0
	github.com/celestiaorg/rsmt2d v0.3.0
	github.com/cosmos/cosmos-sdk v0.44.0
	github.com/cosmos/ibc-go v1.2.0
	github.com/gogo/protobuf v1.3.3
	github.com/ipfs/go-bitswap v0.3.4
	github.com/ipfs/go-block-format v0.0.3
	github.com/ipfs/go-blockservice v0.1.7
	github.com/ipfs/go-cid v0.0.7
	github.com/ipfs/go-datastore v0.4.6
	github.com/ipfs/go-ds-badger2 v0.1.1
	github.com/ipfs/go-ipfs-blockstore v0.1.6
	github.com/ipfs/go-ipfs-exchange-interface v0.0.1
	github.com/ipfs/go-ipld-cbor v0.0.5 // indirect
	github.com/ipfs/go-ipld-format v0.2.0
	github.com/ipfs/go-log/v2 v2.3.0
	github.com/ipfs/go-merkledag v0.3.2
	github.com/ipfs/go-peertaskqueue v0.4.0 // indirect
	github.com/ipld/go-ipld-prime v0.11.0 // indirect
	github.com/libp2p/go-libp2p v0.15.0
	github.com/libp2p/go-libp2p-connmgr v0.2.4
	github.com/libp2p/go-libp2p-core v0.9.0
	github.com/libp2p/go-libp2p-kad-dht v0.13.1
	github.com/libp2p/go-libp2p-peerstore v0.2.8
	github.com/libp2p/go-libp2p-pubsub v0.5.4
	github.com/libp2p/go-libp2p-routing-helpers v0.2.3
	github.com/mitchellh/go-homedir v1.1.0
	github.com/multiformats/go-base32 v0.0.4
	github.com/multiformats/go-multiaddr v0.4.0
	github.com/multiformats/go-multihash v0.0.15
	github.com/spf13/cobra v1.2.1
	github.com/stretchr/testify v1.7.1-0.20210427113832-6241f9ab9942
	github.com/tendermint/spm v0.1.5
	github.com/tendermint/tendermint v0.34.14
	github.com/tendermint/tm-db v0.6.4
	go.uber.org/fx v1.14.2
	go.uber.org/zap v1.19.0
)

replace (
	github.com/99designs/keyring => github.com/cosmos/keyring v1.1.7-0.20210622111912-ef00f8ac3d76
	github.com/celestiaorg/celestia-app => github.com/celestiaorg/lazyledger-app v0.0.1
	github.com/cosmos/cosmos-sdk => github.com/celestiaorg/cosmos-sdk v0.44.1-celestia
	github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1
	github.com/tendermint/tendermint v0.34.14 => github.com/celestiaorg/celestia-core v0.34.14-celestia
	google.golang.org/grpc => google.golang.org/grpc v1.33.2
)
