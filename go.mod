module github.com/celestiaorg/celestia-node

go 1.22.4

require (
	cosmossdk.io/errors v1.0.1
	cosmossdk.io/math v1.3.0
	github.com/BurntSushi/toml v1.4.0
	github.com/alecthomas/jsonschema v0.0.0-20220216202328-9eeeec9d044b
	github.com/benbjohnson/clock v1.3.5
	github.com/celestiaorg/celestia-app/v2 v2.0.0
	github.com/celestiaorg/go-fraud v0.2.1
	github.com/celestiaorg/go-header v0.6.2
	github.com/celestiaorg/go-libp2p-messenger v0.2.0
	github.com/celestiaorg/go-square v1.1.0
	github.com/celestiaorg/go-square/merkle v0.0.0-20240117232118-fd78256df076
	github.com/celestiaorg/nmt v0.22.1
	github.com/celestiaorg/rsmt2d v0.13.1
	github.com/cosmos/cosmos-sdk v0.46.16
	github.com/cristalhq/jwt/v5 v5.4.0
	github.com/dgraph-io/badger/v4 v4.2.1-0.20240106094458-1c417aa3799c
	github.com/etclabscore/go-openrpc-reflect v0.0.37
	github.com/filecoin-project/dagstore v0.5.6
	github.com/filecoin-project/go-jsonrpc v0.4.0
	github.com/gammazero/workerpool v1.1.3
	github.com/gofrs/flock v0.8.1
	github.com/gogo/protobuf v1.3.3
	github.com/golang/mock v1.6.0
	github.com/gorilla/mux v1.8.1
	github.com/grafana/otel-profiling-go v0.5.1
	github.com/grafana/pyroscope-go v1.1.1
	github.com/hashicorp/go-retryablehttp v0.7.7
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/imdario/mergo v0.3.16
	github.com/ipfs/boxo v0.22.0
	github.com/ipfs/go-block-format v0.2.0
	github.com/ipfs/go-cid v0.4.1
	github.com/ipfs/go-datastore v0.6.0
	github.com/ipfs/go-ds-badger4 v0.1.5
	github.com/ipfs/go-ipld-cbor v0.1.0
	github.com/ipfs/go-ipld-format v0.6.0
	github.com/ipfs/go-log/v2 v2.5.1
	github.com/ipld/go-car v0.6.2
	github.com/libp2p/go-libp2p v0.36.2
	github.com/libp2p/go-libp2p-kad-dht v0.25.2
	github.com/libp2p/go-libp2p-pubsub v0.11.0
	github.com/libp2p/go-libp2p-record v0.2.0
	github.com/libp2p/go-libp2p-routing-helpers v0.7.3
	github.com/mitchellh/go-homedir v1.1.0
	github.com/multiformats/go-base32 v0.1.0
	github.com/multiformats/go-multiaddr v0.13.0
	github.com/multiformats/go-multiaddr-dns v0.3.1
	github.com/multiformats/go-multihash v0.2.3
	github.com/open-rpc/meta-schema v0.0.0-20201029221707-1b72ef2ea333
	github.com/prometheus/client_golang v1.19.1
	github.com/rollkit/go-da v0.4.0
	github.com/spf13/cobra v1.8.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.9.0
	github.com/tendermint/tendermint v0.34.29
	go.opentelemetry.io/contrib/instrumentation/runtime v0.45.0
	go.opentelemetry.io/otel v1.27.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.27.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.27.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.27.0
	go.opentelemetry.io/otel/metric v1.27.0
	go.opentelemetry.io/otel/sdk v1.27.0
	go.opentelemetry.io/otel/sdk/metric v1.27.0
	go.opentelemetry.io/otel/trace v1.27.0
	go.opentelemetry.io/proto/otlp v1.3.1
	go.uber.org/fx v1.22.1
	go.uber.org/zap v1.27.0
	golang.org/x/crypto v0.25.0
	golang.org/x/exp v0.0.0-20240719175910-8a7402abbf56
	golang.org/x/sync v0.7.0
	golang.org/x/text v0.16.0
	google.golang.org/grpc v1.64.0
	google.golang.org/protobuf v1.34.2
)

require (
	cloud.google.com/go v0.112.1 // indirect
	cloud.google.com/go/compute/metadata v0.3.0 // indirect
	cloud.google.com/go/iam v1.1.6 // indirect
	cloud.google.com/go/storage v1.38.0 // indirect
	filippo.io/edwards25519 v1.0.0-rc.1 // indirect
	github.com/99designs/go-keychain v0.0.0-20191008050251-8e49817e8af4 // indirect
	github.com/99designs/keyring v1.2.2 // indirect
	github.com/ChainSafe/go-schnorrkel v1.0.0 // indirect
	github.com/Jorropo/jsync v1.0.1 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/StackExchange/wmi v1.2.1 // indirect
	github.com/Workiva/go-datastructures v1.0.53 // indirect
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/aws/aws-sdk-go v1.44.122 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bgentry/go-netrc v0.0.0-20140422174119-9fd32a8b3d3d // indirect
	github.com/bgentry/speakeasy v0.1.0 // indirect
	github.com/bits-and-blooms/bitset v1.10.0 // indirect
	github.com/btcsuite/btcd/btcec/v2 v2.3.2 // indirect
	github.com/celestiaorg/blobstream-contracts/v3 v3.1.0 // indirect
	github.com/celestiaorg/merkletree v0.0.0-20230308153949-c33506a7aa26 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/chzyer/readline v1.5.1 // indirect
	github.com/cockroachdb/apd/v2 v2.0.2 // indirect
	github.com/coinbase/rosetta-sdk-go v0.7.9 // indirect
	github.com/cometbft/cometbft-db v0.7.0 // indirect
	github.com/confio/ics23/go v0.9.1 // indirect
	github.com/consensys/bavard v0.1.13 // indirect
	github.com/consensys/gnark-crypto v0.12.1 // indirect
	github.com/containerd/cgroups v1.1.0 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/cosmos/btcutil v1.0.5 // indirect
	github.com/cosmos/cosmos-proto v1.0.0-beta.5 // indirect
	github.com/cosmos/go-bip39 v1.0.0 // indirect
	github.com/cosmos/gogoproto v1.5.0 // indirect
	github.com/cosmos/gorocksdb v1.2.0 // indirect
	github.com/cosmos/iavl v0.19.6 // indirect
	github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v6 v6.1.2 // indirect
	github.com/cosmos/ibc-go/v6 v6.3.0 // indirect
	github.com/cosmos/ledger-cosmos-go v0.13.2 // indirect
	github.com/crate-crypto/go-kzg-4844 v1.0.0 // indirect
	github.com/creachadair/taskgroup v0.3.2 // indirect
	github.com/cskr/pubsub v1.0.2 // indirect
	github.com/danieljoos/wincred v1.1.2 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/davidlazar/go-crypto v0.0.0-20200604182044-b73af7476f6c // indirect
	github.com/deckarep/golang-set/v2 v2.6.0 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.3.0 // indirect
	github.com/desertbit/timer v0.0.0-20180107155436-c41aec40b27f // indirect
	github.com/dgraph-io/badger/v2 v2.2007.4 // indirect
	github.com/dgraph-io/ristretto v0.1.1 // indirect
	github.com/dgryski/go-farm v0.0.0-20200201041132-a6ae2369ad13 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/dvsekhvalnov/jose2go v1.6.0 // indirect
	github.com/elastic/gosigar v0.14.3 // indirect
	github.com/etclabscore/go-jsonschema-walk v0.0.6 // indirect
	github.com/ethereum/c-kzg-4844 v1.0.0 // indirect
	github.com/ethereum/go-ethereum v1.14.5 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/flynn/noise v1.1.0 // indirect
	github.com/francoispqt/gojay v1.2.13 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/gammazero/deque v0.2.0 // indirect
	github.com/go-kit/kit v0.12.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/spec v0.19.11 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/godbus/dbus v0.0.0-20190726142602-4481cbc300e2 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gogo/gateway v1.1.0 // indirect
	github.com/golang/glog v1.2.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.5-0.20220116011046-fa5810519dcb // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/google/flatbuffers v1.12.1 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gopacket v1.1.19 // indirect
	github.com/google/orderedcode v0.0.1 // indirect
	github.com/google/pprof v0.0.0-20240727154555-813a5fbdbec8 // indirect
	github.com/google/s2a-go v0.1.7 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.2 // indirect
	github.com/googleapis/gax-go/v2 v2.12.2 // indirect
	github.com/gorilla/handlers v1.5.2 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/grafana/pyroscope-go/godeltaprof v0.1.6 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.20.0 // indirect
	github.com/gsterjov/go-libsecret v0.0.0-20161001094733-a6f4afe4910c // indirect
	github.com/gtank/merlin v0.1.1 // indirect
	github.com/gtank/ristretto255 v0.1.2 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-getter v1.7.5 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-safetemp v1.0.0 // indirect
	github.com/hashicorp/go-version v1.6.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/hashicorp/golang-lru/arc/v2 v2.0.7 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hdevalence/ed25519consensus v0.0.0-20220222234857-c00d1f31bab3 // indirect
	github.com/holiman/uint256 v1.2.4 // indirect
	github.com/huin/goupnp v1.3.0 // indirect
	github.com/iancoleman/orderedmap v0.2.0 // indirect
	github.com/improbable-eng/grpc-web v0.15.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/ipfs/bbloom v0.0.4 // indirect
	github.com/ipfs/go-blockservice v0.5.2 // indirect
	github.com/ipfs/go-ipfs-blockstore v1.3.1 // indirect
	github.com/ipfs/go-ipfs-delay v0.0.1 // indirect
	github.com/ipfs/go-ipfs-ds-help v1.1.1 // indirect
	github.com/ipfs/go-ipfs-exchange-interface v0.2.1 // indirect
	github.com/ipfs/go-ipfs-pq v0.0.3 // indirect
	github.com/ipfs/go-ipfs-util v0.0.3 // indirect
	github.com/ipfs/go-ipld-legacy v0.2.1 // indirect
	github.com/ipfs/go-log v1.0.5 // indirect
	github.com/ipfs/go-merkledag v0.11.0 // indirect
	github.com/ipfs/go-metrics-interface v0.0.1 // indirect
	github.com/ipfs/go-peertaskqueue v0.8.1 // indirect
	github.com/ipfs/go-verifcid v0.0.3 // indirect
	github.com/ipld/go-car/v2 v2.13.1 // indirect
	github.com/ipld/go-codec-dagpb v1.6.0 // indirect
	github.com/ipld/go-ipld-prime v0.21.0 // indirect
	github.com/jackpal/go-nat-pmp v1.0.2 // indirect
	github.com/jbenet/go-temp-err-catcher v0.1.0 // indirect
	github.com/jbenet/goprocess v0.1.4 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/jmhodges/levigo v1.0.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/klauspost/cpuid/v2 v2.2.8 // indirect
	github.com/klauspost/reedsolomon v1.12.1 // indirect
	github.com/koron/go-ssdp v0.0.4 // indirect
	github.com/lib/pq v1.10.7 // indirect
	github.com/libp2p/go-buffer-pool v0.1.0 // indirect
	github.com/libp2p/go-cidranger v1.1.0 // indirect
	github.com/libp2p/go-flow-metrics v0.1.0 // indirect
	github.com/libp2p/go-libp2p-asn-util v0.4.1 // indirect
	github.com/libp2p/go-libp2p-kbucket v0.6.3 // indirect
	github.com/libp2p/go-msgio v0.3.0 // indirect
	github.com/libp2p/go-nat v0.2.0 // indirect
	github.com/libp2p/go-netroute v0.2.1 // indirect
	github.com/libp2p/go-reuseport v0.4.0 // indirect
	github.com/libp2p/go-yamux/v4 v4.0.1 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/manifoldco/promptui v0.9.0 // indirect
	github.com/marten-seemann/tcp v0.0.0-20210406111302-dfbc87cc63fd // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/miekg/dns v1.1.61 // indirect
	github.com/mikioh/tcpinfo v0.0.0-20190314235526-30a79bb1804b // indirect
	github.com/mikioh/tcpopt v0.0.0-20190314235656-172688c1accc // indirect
	github.com/mimoo/StrobeGo v0.0.0-20210601165009-122bf33a46e0 // indirect
	github.com/minio/highwayhash v1.0.2 // indirect
	github.com/minio/sha256-simd v1.0.1 // indirect
	github.com/mitchellh/go-testing-interface v1.14.1 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mmcloughlin/addchain v0.4.0 // indirect
	github.com/mr-tron/base58 v1.2.0 // indirect
	github.com/mtibben/percent v0.2.1 // indirect
	github.com/multiformats/go-base36 v0.2.0 // indirect
	github.com/multiformats/go-multiaddr-fmt v0.1.0 // indirect
	github.com/multiformats/go-multibase v0.2.0 // indirect
	github.com/multiformats/go-multicodec v0.9.0 // indirect
	github.com/multiformats/go-multistream v0.5.0 // indirect
	github.com/multiformats/go-varint v0.0.7 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/onsi/ginkgo/v2 v2.19.1 // indirect
	github.com/opencontainers/runtime-spec v1.2.0 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58 // indirect
	github.com/pelletier/go-toml/v2 v2.1.0 // indirect
	github.com/petar/GoLLRB v0.0.0-20210522233825-ae3b015fd3e9 // indirect
	github.com/petermattis/goid v0.0.0-20230317030725-371a4b8eda08 // indirect
	github.com/pion/datachannel v1.5.8 // indirect
	github.com/pion/dtls/v2 v2.2.12 // indirect
	github.com/pion/ice/v2 v2.3.34 // indirect
	github.com/pion/interceptor v0.1.29 // indirect
	github.com/pion/logging v0.2.2 // indirect
	github.com/pion/mdns v0.0.12 // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/rtcp v1.2.14 // indirect
	github.com/pion/rtp v1.8.8 // indirect
	github.com/pion/sctp v1.8.20 // indirect
	github.com/pion/sdp/v3 v3.0.9 // indirect
	github.com/pion/srtp/v2 v2.0.20 // indirect
	github.com/pion/stun v0.6.1 // indirect
	github.com/pion/transport/v2 v2.2.10 // indirect
	github.com/pion/turn/v2 v2.1.6 // indirect
	github.com/pion/webrtc/v3 v3.3.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/polydawn/refmt v0.89.0 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.55.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/quic-go/qpack v0.4.0 // indirect
	github.com/quic-go/quic-go v0.45.2 // indirect
	github.com/quic-go/webtransport-go v0.8.0 // indirect
	github.com/rakyll/statik v0.1.7 // indirect
	github.com/raulk/go-watchdog v1.3.0 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475 // indirect
	github.com/regen-network/cosmos-proto v0.3.1 // indirect
	github.com/rs/cors v1.8.3 // indirect
	github.com/rs/zerolog v1.33.0 // indirect
	github.com/sasha-s/go-deadlock v0.3.1 // indirect
	github.com/shirou/gopsutil v3.21.6+incompatible // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/spf13/afero v1.11.0 // indirect
	github.com/spf13/cast v1.6.0 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/viper v1.15.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/supranational/blst v0.3.11 // indirect
	github.com/syndtr/goleveldb v1.0.1-0.20220721030215-126854af5e6d // indirect
	github.com/tecbot/gorocksdb v0.0.0-20191217155057-f0fad39f321c // indirect
	github.com/tendermint/go-amino v0.16.0 // indirect
	github.com/tendermint/tm-db v0.6.7 // indirect
	github.com/tidwall/btree v1.5.0 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/ulikunitz/xz v0.5.10 // indirect
	github.com/whyrusleeping/cbor v0.0.0-20171005072247-63513f603b11 // indirect
	github.com/whyrusleeping/cbor-gen v0.1.2 // indirect
	github.com/whyrusleeping/go-keyspace v0.0.0-20160322163242-5b898ac5add1 // indirect
	github.com/wlynxg/anet v0.0.3 // indirect
	github.com/zondax/hid v0.9.2 // indirect
	github.com/zondax/ledger-go v0.14.3 // indirect
	go.etcd.io/bbolt v1.3.6 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.49.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.52.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.27.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/dig v1.17.1 // indirect
	go.uber.org/mock v0.4.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/mod v0.19.0 // indirect
	golang.org/x/net v0.27.0 // indirect
	golang.org/x/oauth2 v0.21.0 // indirect
	golang.org/x/sys v0.22.0 // indirect
	golang.org/x/term v0.22.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	golang.org/x/tools v0.23.0 // indirect
	golang.org/x/xerrors v0.0.0-20231012003039-104605ab7028 // indirect
	gonum.org/v1/gonum v0.15.0 // indirect
	google.golang.org/api v0.169.0 // indirect
	google.golang.org/genproto v0.0.0-20240227224415-6ceb2ff114de // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240617180043-68d350f18fd4 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240617180043-68d350f18fd4 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	lukechampine.com/blake3 v1.3.0 // indirect
	nhooyr.io/websocket v1.8.7 // indirect
	rsc.io/tmplfunc v0.0.3 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

replace (
	github.com/cosmos/cosmos-sdk => github.com/celestiaorg/cosmos-sdk v1.23.0-sdk-v0.46.16
	github.com/filecoin-project/dagstore => github.com/celestiaorg/dagstore v0.0.0-20230824094345-537c012aa403
	github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1
	// broken goleveldb needs to be replaced for the cosmos-sdk and celestia-app
	github.com/syndtr/goleveldb => github.com/syndtr/goleveldb v1.0.1-0.20210819022825-2ae1ddf74ef7
	github.com/tendermint/tendermint => github.com/celestiaorg/celestia-core v1.38.0-tm-v0.34.29
)
