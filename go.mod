module github.com/celestiaorg/celestia-node

go 1.17

replace github.com/ipfs/go-verifcid => github.com/celestiaorg/go-verifcid v0.0.1-lazypatch

require (
	github.com/BurntSushi/toml v1.1.0
	github.com/celestiaorg/celestia-app v0.4.0
	github.com/celestiaorg/go-libp2p-messenger v0.1.0
	github.com/celestiaorg/nmt v0.8.0
	github.com/celestiaorg/rsmt2d v0.4.0
	github.com/cosmos/cosmos-sdk v0.44.3
	github.com/dgraph-io/badger/v2 v2.2007.4
	github.com/gogo/protobuf v1.3.3
	github.com/gorilla/mux v1.8.0
	github.com/hashicorp/go-retryablehttp v0.7.1-0.20211018174820-ff6d014e72d9
	github.com/hashicorp/golang-lru v0.5.4
	github.com/ipfs/go-bitswap v0.4.0
	github.com/ipfs/go-block-format v0.0.3
	github.com/ipfs/go-blockservice v0.1.7
	github.com/ipfs/go-cid v0.1.0
	github.com/ipfs/go-datastore v0.4.6
	github.com/ipfs/go-ds-badger2 v0.1.1
	github.com/ipfs/go-ipfs-blockstore v0.1.6
	github.com/ipfs/go-ipfs-exchange-interface v0.0.1
	github.com/ipfs/go-ipfs-exchange-offline v0.0.1
	github.com/ipfs/go-ipfs-routing v0.1.0
	github.com/ipfs/go-ipld-format v0.2.0
	github.com/ipfs/go-log/v2 v2.5.1
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
	github.com/multiformats/go-multiaddr v0.5.0
	github.com/multiformats/go-multihash v0.1.0
	github.com/raulk/go-watchdog v1.2.0
	github.com/spf13/cobra v1.4.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.1-0.20210427113832-6241f9ab9942
	github.com/tendermint/spm v0.1.9
	github.com/tendermint/tendermint v0.34.16
	go.uber.org/fx v1.17.1
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	google.golang.org/grpc v1.46.0
)

require (
	filippo.io/edwards25519 v1.0.0-beta.2 // indirect
	github.com/99designs/keyring v1.1.6 // indirect
	github.com/ChainSafe/go-schnorrkel v0.0.0-20200405005733-88cbf1b4c40d // indirect
	github.com/Workiva/go-datastructures v1.0.53 // indirect
	github.com/armon/go-metrics v0.3.10 // indirect
	github.com/benbjohnson/clock v1.3.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bgentry/speakeasy v0.1.0 // indirect
	github.com/btcsuite/btcd v0.22.0-beta // indirect
	github.com/celestiaorg/go-leopard v0.1.0 // indirect
	github.com/celestiaorg/merkletree v0.0.0-20210714075610-a84dc3ddbbe4 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/coinbase/rosetta-sdk-go v0.6.10 // indirect
	github.com/confio/ics23/go v0.6.6 // indirect
	github.com/containerd/cgroups v0.0.0-20201119153540-4cbc285b3327 // indirect
	github.com/coreos/go-systemd/v22 v22.3.2 // indirect
	github.com/cosmos/go-bip39 v1.0.0 // indirect
	github.com/cosmos/iavl v0.17.1 // indirect
	github.com/cosmos/ibc-go v1.2.2 // indirect
	github.com/cosmos/ledger-cosmos-go v0.11.1 // indirect
	github.com/cosmos/ledger-go v0.9.2 // indirect
	github.com/cskr/pubsub v1.0.2 // indirect
	github.com/danieljoos/wincred v1.0.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/davidlazar/go-crypto v0.0.0-20200604182044-b73af7476f6c // indirect
	github.com/desertbit/timer v0.0.0-20180107155436-c41aec40b27f // indirect
	github.com/dgraph-io/ristretto v0.0.3 // indirect
	github.com/dgryski/go-farm v0.0.0-20200201041132-a6ae2369ad13 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/dvsekhvalnov/jose2go v0.0.0-20200901110807-248326c1351b // indirect
	github.com/elastic/gosigar v0.12.0 // indirect
	github.com/enigmampc/btcutil v1.0.3-0.20200723161021-e2fb6adb2a25 // indirect
	github.com/felixge/httpsnoop v1.0.1 // indirect
	github.com/flynn/noise v1.0.0 // indirect
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/go-kit/kit v0.12.0 // indirect
	github.com/go-kit/log v0.2.0 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/godbus/dbus v0.0.0-20190726142602-4481cbc300e2 // indirect
	github.com/godbus/dbus/v5 v5.0.4 // indirect
	github.com/gogo/gateway v1.1.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.3 // indirect
	github.com/google/btree v1.0.0 // indirect
	github.com/google/gopacket v1.1.19 // indirect
	github.com/google/orderedcode v0.0.1 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/gorilla/handlers v1.5.1 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/gsterjov/go-libsecret v0.0.0-20161001094733-a6f4afe4910c // indirect
	github.com/gtank/merlin v0.1.1 // indirect
	github.com/gtank/ristretto255 v0.1.2 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hdevalence/ed25519consensus v0.0.0-20210204194344-59a8610d2b87 // indirect
	github.com/huin/goupnp v1.0.2 // indirect
	github.com/improbable-eng/grpc-web v0.14.1 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/ipfs/bbloom v0.0.4 // indirect
	github.com/ipfs/go-ipfs-delay v0.0.1 // indirect
	github.com/ipfs/go-ipfs-ds-help v0.1.1 // indirect
	github.com/ipfs/go-ipfs-pq v0.0.2 // indirect
	github.com/ipfs/go-ipfs-util v0.0.2 // indirect
	github.com/ipfs/go-ipld-cbor v0.0.3 // indirect
	github.com/ipfs/go-ipns v0.1.2 // indirect
	github.com/ipfs/go-log v1.0.5 // indirect
	github.com/ipfs/go-metrics-interface v0.0.1 // indirect
	github.com/ipfs/go-peertaskqueue v0.4.0 // indirect
	github.com/ipfs/go-verifcid v0.0.1 // indirect
	github.com/ipld/go-ipld-prime v0.9.0 // indirect
	github.com/jackpal/go-nat-pmp v1.0.2 // indirect
	github.com/jbenet/go-temp-err-catcher v0.1.0 // indirect
	github.com/jbenet/goprocess v0.1.4 // indirect
	github.com/jmhodges/levigo v1.0.0 // indirect
	github.com/keybase/go-keychain v0.0.0-20190712205309-48d3d31d256d // indirect
	github.com/klauspost/compress v1.13.6 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/koron/go-ssdp v0.0.2 // indirect
	github.com/lib/pq v1.10.4 // indirect
	github.com/libp2p/go-addr-util v0.1.0 // indirect
	github.com/libp2p/go-buffer-pool v0.0.2 // indirect
	github.com/libp2p/go-cidranger v1.1.0 // indirect
	github.com/libp2p/go-conn-security-multistream v0.2.1 // indirect
	github.com/libp2p/go-eventbus v0.2.1 // indirect
	github.com/libp2p/go-flow-metrics v0.0.3 // indirect
	github.com/libp2p/go-libp2p-asn-util v0.0.0-20200825225859-85005c6cf052 // indirect
	github.com/libp2p/go-libp2p-autonat v0.4.2 // indirect
	github.com/libp2p/go-libp2p-blankhost v0.2.0 // indirect
	github.com/libp2p/go-libp2p-circuit v0.4.0 // indirect
	github.com/libp2p/go-libp2p-discovery v0.5.1 // indirect
	github.com/libp2p/go-libp2p-kbucket v0.4.7 // indirect
	github.com/libp2p/go-libp2p-loggables v0.1.0 // indirect
	github.com/libp2p/go-libp2p-mplex v0.4.1 // indirect
	github.com/libp2p/go-libp2p-nat v0.0.6 // indirect
	github.com/libp2p/go-libp2p-netutil v0.1.0 // indirect
	github.com/libp2p/go-libp2p-noise v0.2.2 // indirect
	github.com/libp2p/go-libp2p-pnet v0.2.0 // indirect
	github.com/libp2p/go-libp2p-swarm v0.5.3 // indirect
	github.com/libp2p/go-libp2p-testing v0.4.2 // indirect
	github.com/libp2p/go-libp2p-tls v0.2.0 // indirect
	github.com/libp2p/go-libp2p-transport-upgrader v0.4.6 // indirect
	github.com/libp2p/go-libp2p-yamux v0.5.4 // indirect
	github.com/libp2p/go-maddr-filter v0.1.0 // indirect
	github.com/libp2p/go-mplex v0.3.0 // indirect
	github.com/libp2p/go-msgio v0.0.6 // indirect
	github.com/libp2p/go-nat v0.0.5 // indirect
	github.com/libp2p/go-netroute v0.1.6 // indirect
	github.com/libp2p/go-openssl v0.0.7 // indirect
	github.com/libp2p/go-reuseport v0.0.2 // indirect
	github.com/libp2p/go-reuseport-transport v0.0.5 // indirect
	github.com/libp2p/go-sockaddr v0.1.1 // indirect
	github.com/libp2p/go-stream-muxer-multistream v0.3.0 // indirect
	github.com/libp2p/go-tcp-transport v0.2.8 // indirect
	github.com/libp2p/go-ws-transport v0.5.0 // indirect
	github.com/libp2p/go-yamux/v2 v2.2.0 // indirect
	github.com/magiconair/properties v1.8.5 // indirect
	github.com/marten-seemann/tcp v0.0.0-20210406111302-dfbc87cc63fd // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/miekg/dns v1.1.43 // indirect
	github.com/mikioh/tcpinfo v0.0.0-20190314235526-30a79bb1804b // indirect
	github.com/mikioh/tcpopt v0.0.0-20190314235656-172688c1accc // indirect
	github.com/mimoo/StrobeGo v0.0.0-20181016162300-f8f6d4d2b643 // indirect
	github.com/minio/highwayhash v1.0.2 // indirect
	github.com/minio/sha256-simd v1.0.0 // indirect
	github.com/mitchellh/mapstructure v1.4.3 // indirect
	github.com/mr-tron/base58 v1.2.0 // indirect
	github.com/mtibben/percent v0.2.1 // indirect
	github.com/multiformats/go-base36 v0.1.0 // indirect
	github.com/multiformats/go-multiaddr-dns v0.3.1 // indirect
	github.com/multiformats/go-multiaddr-fmt v0.1.0 // indirect
	github.com/multiformats/go-multibase v0.0.3 // indirect
	github.com/multiformats/go-multicodec v0.2.0 // indirect
	github.com/multiformats/go-multistream v0.2.2 // indirect
	github.com/multiformats/go-varint v0.0.6 // indirect
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/petermattis/goid v0.0.0-20180202154549-b0b1615b78e5 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/polydawn/refmt v0.0.0-20190807091052-3d65705ee9f1 // indirect
	github.com/prometheus/client_golang v1.12.1 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.32.1 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/rakyll/statik v0.1.7 // indirect
	github.com/raulk/clock v1.1.0 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20200313005456-10cdbea86bc0 // indirect
	github.com/regen-network/cosmos-proto v0.3.1 // indirect
	github.com/rs/cors v1.8.2 // indirect
	github.com/rs/zerolog v1.23.0 // indirect
	github.com/sasha-s/go-deadlock v0.2.1-0.20190427202633-1595213edefa // indirect
	github.com/spacemonkeygo/spacelog v0.0.0-20180420211403-2296661a0572 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/spf13/afero v1.6.0 // indirect
	github.com/spf13/cast v1.4.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/viper v1.10.1 // indirect
	github.com/subosito/gotenv v1.2.0 // indirect
	github.com/syndtr/goleveldb v1.0.1-0.20200815110645-5c35d600f0ca // indirect
	github.com/tecbot/gorocksdb v0.0.0-20191217155057-f0fad39f321c // indirect
	github.com/tendermint/btcd v0.1.1 // indirect
	github.com/tendermint/crypto v0.0.0-20191022145703-50d29ede1e15 // indirect
	github.com/tendermint/go-amino v0.16.0 // indirect
	github.com/tendermint/tm-db v0.6.6 // indirect
	github.com/vivint/infectious v0.0.0-20200605153912-25a574ae18a3 // indirect
	github.com/whyrusleeping/go-keyspace v0.0.0-20160322163242-5b898ac5add1 // indirect
	github.com/whyrusleeping/multiaddr-filter v0.0.0-20160516205228-e903e4adabd7 // indirect
	github.com/whyrusleeping/timecache v0.0.0-20160911033111-cfcb2f1abfee // indirect
	github.com/zondax/hid v0.9.0 // indirect
	go.etcd.io/bbolt v1.3.6 // indirect
	go.opencensus.io v0.23.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/dig v1.14.0 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	go.uber.org/zap v1.21.0 // indirect
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519 // indirect
	golang.org/x/net v0.0.0-20211208012354-db4efeb81f4b // indirect
	golang.org/x/sys v0.0.0-20220114195835-da31bd327af9 // indirect
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211 // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/genproto v0.0.0-20211208223120-3a66f561d7aa // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	gopkg.in/ini.v1 v1.66.2 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	lukechampine.com/blake3 v1.1.6 // indirect
	nhooyr.io/websocket v1.8.6 // indirect
)

replace (
	github.com/cosmos/cosmos-sdk => github.com/celestiaorg/cosmos-sdk v0.44.2-celestia-rc-1
	github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1
	github.com/libp2p/go-libp2p-pubsub v0.5.7-0.20211029175501-5c90105738cf => github.com/celestiaorg/go-libp2p-pubsub v0.5.6-celestia
	github.com/tendermint/tendermint => github.com/celestiaorg/celestia-core v1.1.0-tm-v0.34.16
)
