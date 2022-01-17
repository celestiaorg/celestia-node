module github.com/celestiaorg/celestia-node

go 1.16

replace github.com/ipfs/go-verifcid => github.com/celestiaorg/go-verifcid v0.0.1-lazypatch

require (
	github.com/BurntSushi/toml v0.4.1
	github.com/celestiaorg/go-libp2p-messenger v0.1.0
	github.com/celestiaorg/nmt v0.8.0
	github.com/celestiaorg/rsmt2d v0.3.0
	github.com/gogo/protobuf v1.3.2
	github.com/hashicorp/golang-lru v0.5.4
	github.com/ipfs/go-bitswap v0.4.0
	github.com/ipfs/go-block-format v0.0.3
	github.com/ipfs/go-blockservice v0.1.7
	github.com/ipfs/go-cid v0.1.0
	github.com/ipfs/go-datastore v0.5.0
	github.com/ipfs/go-ds-badger2 v0.1.2
	github.com/ipfs/go-ipfs-blockstore v0.1.6
	github.com/ipfs/go-ipfs-exchange-interface v0.0.1
	github.com/ipfs/go-ipfs-routing v0.1.0
	github.com/ipfs/go-ipld-format v0.2.0
	github.com/ipfs/go-log/v2 v2.4.0
	github.com/ipfs/go-merkledag v0.3.2
	github.com/libp2p/go-libp2p v0.15.1
	github.com/libp2p/go-libp2p-connmgr v0.2.4
	github.com/libp2p/go-libp2p-core v0.9.0
	github.com/libp2p/go-libp2p-kad-dht v0.14.0
	github.com/libp2p/go-libp2p-peerstore v0.3.0
	github.com/libp2p/go-libp2p-pubsub v0.5.7-0.20211029175501-5c90105738cf
	github.com/libp2p/go-libp2p-record v0.1.3
	github.com/libp2p/go-libp2p-routing-helpers v0.2.3
	github.com/minio/blake2b-simd v0.0.0-20160723061019-3f5f724cb5b1
	github.com/mitchellh/go-homedir v1.1.0
	github.com/multiformats/go-base32 v0.0.4
	github.com/multiformats/go-multiaddr v0.4.1
	github.com/multiformats/go-multihash v0.1.0
	github.com/spf13/cobra v1.3.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.1-0.20210427113832-6241f9ab9942
	github.com/tendermint/tendermint v0.34.14
	go.uber.org/fx v1.16.0
	go.uber.org/zap v1.19.0
)

replace github.com/tendermint/tendermint v0.34.14 => github.com/celestiaorg/celestia-core v0.34.14-celestia
