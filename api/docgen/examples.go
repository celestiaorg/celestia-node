package docgen

import (
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/multiformats/go-multiaddr"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/celestiaorg/go-fraud"
	libhead "github.com/celestiaorg/go-header"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
	"github.com/celestiaorg/celestia-node/state"
)

var (
	//go:embed "exampledata/extendedHeader.json"
	exampleExtendedHeader string

	//go:embed "exampledata/samplingStats.json"
	exampleSamplingStats string

	//go:embed "exampledata/txResponse.json"
	exampleTxResponse string

	//go:embed "exampledata/resourceManagerStats.json"
	exampleResourceMngrStats string

	//go:embed "exampledata/blob.json"
	exampleBlob string

	//go:embed "exampledata/blobProof.json"
	exampleBlobProof string
)

var exampleValues = map[reflect.Type]any{}

func add(v any) {
	typ := reflect.TypeOf(v)
	exampleValues[typ] = v
}

func init() {
	add("string value")
	add(uint64(42))
	add(uint32(42))
	add(int32(42))
	add(int64(42))
	add(42)
	add(byte(7))
	add(float64(42))
	add(float64(42))
	add(true)
	add([]byte("byte array"))
	add(time.Second)
	add(node.Full)
	add(auth.Permission("admin"))
	add(byzantine.BadEncoding)

	// TODO: this case requires more debugging, simple to leave it as it was.
	exampleValues[reflect.TypeOf((*fraud.Proof[*header.ExtendedHeader])(nil)).Elem()] = byzantine.CreateBadEncodingProof(
		[]byte("bad encoding proof"),
		42,
		&byzantine.ErrByzantine{
			Index:  0,
			Shares: []*byzantine.ShareWithProof{},
			Axis:   rsmt2d.Axis(0),
		},
	)

	add(errors.New("error"))
	add(state.Balance{Amount: sdk.NewInt(42), Denom: "utia"})
	add(share.EmptyEDS())
	add(rsmt2d.Row)
	add(network.Connected)
	add(network.ReachabilityPrivate)

	addr := must(sdk.AccAddressFromBech32("celestia1377k5an3f94v6wyaceu0cf4nq6gk2jtpc46g7h"))
	add(addr)
	add(state.Address{Address: addr})
	exampleValues[reflect.TypeOf((*sdk.Address)(nil)).Elem()] = addr

	valAddr := must(sdk.ValAddressFromBech32("celestiavaloper1q3v5cugc8cdpud87u4zwy0a74uxkk6u4q4gx4p"))
	add(valAddr)

	var txResponse *state.TxResponse
	err := json.Unmarshal([]byte(exampleTxResponse), &txResponse)
	if err != nil {
		panic(err)
	}
	add(txResponse)

	var samplingStats das.SamplingStats
	err = json.Unmarshal([]byte(exampleSamplingStats), &samplingStats)
	if err != nil {
		panic(err)
	}
	add(samplingStats)

	var extendedHeader *header.ExtendedHeader
	err = json.Unmarshal([]byte(exampleExtendedHeader), &extendedHeader)
	if err != nil {
		panic(err)
	}
	add(extendedHeader)

	var resourceMngrStats rcmgr.ResourceManagerStat
	err = json.Unmarshal([]byte(exampleResourceMngrStats), &resourceMngrStats)
	if err != nil {
		panic(err)
	}
	add(resourceMngrStats)

	var exBlob *blob.Blob
	err = json.Unmarshal([]byte(exampleBlob), &exBlob)
	if err != nil {
		panic(err)
	}
	add(exBlob)
	add(exBlob.Blob)

	var blobProof *blob.Proof
	err = json.Unmarshal([]byte(exampleBlobProof), &blobProof)
	if err != nil {
		panic(err)
	}
	add(blobProof)

	mathInt, _ := math.NewIntFromString("42")
	add(mathInt)

	pID := protocol.ID("/celestia/mocha/ipfs/bitswap")
	add(pID)

	peerID := peer.ID("12D3KooWPRb5h3g9MH7sx9qfbSQZG5cXv1a2Qs3o4aW5YmmzPq82")
	add(peerID)

	ma, _ := multiaddr.NewMultiaddr("/ip6/::1/udp/2121/quic-v1")
	addrInfo := peer.AddrInfo{
		ID:    peerID,
		Addrs: []multiaddr.Multiaddr{ma},
	}
	add(addrInfo)

	commitment := must(base64.StdEncoding.DecodeString("aHlbp+J9yub6hw/uhK6dP8hBLR2mFy78XNRRdLf2794="))
	add(blob.Commitment(commitment))

	// randomly generated namespace that's used in the blob example above
	// (AAAAAAAAAAAAAAAAAAAAAAAAAAAAAMJ/xGlNMdE=)
	namespace := must(libshare.NewV0Namespace([]byte{0xc2, 0x7f, 0xc4, 0x69, 0x4d, 0x31, 0xd1}))
	add(namespace)

	hashStr := "453D0BC3CB88A2ED6F2E06021383B22C72D25D7741AE51B4CAE1AD34D72A3F07"
	hash := must(hex.DecodeString(hashStr))
	add(libhead.Hash(hash))

	add(state.NewTxConfig(
		state.WithGasPrice(0.002),
		state.WithGas(142225),
		state.WithKeyName("my_celes_key"),
		state.WithSignerAddress("celestia1pjcmwj8w6hyr2c4wehakc5g8cfs36aysgucx66"),
		state.WithFeeGranterAddress("celestia1hakc56ax66ypjcmwj8w6hyr2c4g8cfs3wesguc"),
		state.WithMaxGasPrice(state.DefaultMaxGasPrice),
		state.WithTxPriority(1),
	))

	add(network.DirUnknown)
}

func exampleValue(t, parent reflect.Type) (any, error) {
	v, ok := exampleValues[t]
	if ok {
		return v, nil
	}

	switch t.Kind() {
	case reflect.Slice:
		out := reflect.New(t).Elem()
		val, err := exampleValue(t.Elem(), t)
		if err != nil {
			return nil, err
		}
		out = reflect.Append(out, reflect.ValueOf(val))
		return out.Interface(), nil
	case reflect.Chan:
		return exampleValue(t.Elem(), nil)
	case reflect.Struct:
		es, err := exampleStruct(t, parent)
		if err != nil {
			return nil, err
		}
		v := reflect.ValueOf(es).Elem().Interface()
		exampleValues[t] = v
		return v, nil
	case reflect.Array:
		out := reflect.New(t).Elem()
		for i := 0; i < t.Len(); i++ {
			val, err := exampleValue(t.Elem(), t)
			if err != nil {
				return nil, err
			}
			out.Index(i).Set(reflect.ValueOf(val))
		}
		return out.Interface(), nil

	case reflect.Ptr:
		if t.Elem().Kind() == reflect.Struct {
			es, err := exampleStruct(t.Elem(), t)
			if err != nil {
				return nil, err
			}
			return es, err
		}
	case reflect.Interface:
		return struct{}{}, nil
	}
	return nil, fmt.Errorf("failed to retrieve example value for type: %s on parent '%s')", t, parent)
}

func exampleStruct(t, parent reflect.Type) (any, error) {
	ns := reflect.New(t)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Type == parent {
			continue
		}
		if cases.Title(language.Und, cases.NoLower).String(f.Name) == f.Name {
			val, err := exampleValue(f.Type, t)
			if err != nil {
				return nil, err
			}
			ns.Elem().Field(i).Set(reflect.ValueOf(val))
		}
	}

	return ns.Interface(), nil
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
