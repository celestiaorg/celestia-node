package docgen

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"reflect"

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
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/blob"
	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/nodebuilder/node"
	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
	"github.com/celestiaorg/celestia-node/state"
)

//go:embed "exampledata/extendedHeader.json"
var exampleExtendedHeader string

//go:embed "exampledata/samplingStats.json"
var exampleSamplingStats string

//go:embed "exampledata/txResponse.json"
var exampleTxResponse string

//go:embed "exampledata/resourceManagerStats.json"
var exampleResourceMngrStats string

var ExampleValues = map[reflect.Type]interface{}{
	reflect.TypeOf(""):                       "string value",
	reflect.TypeOf(uint64(42)):               uint64(42),
	reflect.TypeOf(uint32(42)):               uint32(42),
	reflect.TypeOf(int32(42)):                int32(42),
	reflect.TypeOf(int64(42)):                int64(42),
	reflect.TypeOf(42):                       42,
	reflect.TypeOf(byte(7)):                  byte(7),
	reflect.TypeOf(float64(42)):              float64(42),
	reflect.TypeOf(true):                     true,
	reflect.TypeOf([]byte{}):                 []byte("byte array"),
	reflect.TypeOf(node.Full):                node.Full,
	reflect.TypeOf(auth.Permission("admin")): auth.Permission("admin"),
	reflect.TypeOf(byzantine.BadEncoding):    byzantine.BadEncoding,
	reflect.TypeOf((*fraud.Proof)(nil)).Elem(): byzantine.CreateBadEncodingProof(
		[]byte("bad encoding proof"),
		42,
		&byzantine.ErrByzantine{
			Index:  0,
			Axis:   rsmt2d.Axis(0),
			Shares: []*byzantine.ShareWithProof{},
		},
	),
	reflect.TypeOf((*error)(nil)).Elem(): fmt.Errorf("error"),
}

func init() {
	addToExampleValues(share.EmptyExtendedDataSquare())
	addr, err := sdk.AccAddressFromBech32("celestia1377k5an3f94v6wyaceu0cf4nq6gk2jtpc46g7h")
	if err != nil {
		panic(err)
	}
	addToExampleValues(addr)
	ExampleValues[reflect.TypeOf((*sdk.Address)(nil)).Elem()] = addr

	valAddr, err := sdk.ValAddressFromBech32("celestiavaloper1q3v5cugc8cdpud87u4zwy0a74uxkk6u4q4gx4p")
	if err != nil {
		panic(err)
	}
	addToExampleValues(valAddr)

	addToExampleValues(state.Address{Address: addr})

	var txResponse *state.TxResponse
	err = json.Unmarshal([]byte(exampleTxResponse), &txResponse)
	if err != nil {
		panic(err)
	}

	var samplingStats das.SamplingStats
	err = json.Unmarshal([]byte(exampleSamplingStats), &samplingStats)
	if err != nil {
		panic(err)
	}

	var extendedHeader *header.ExtendedHeader
	err = json.Unmarshal([]byte(exampleExtendedHeader), &extendedHeader)
	if err != nil {
		panic(err)
	}

	var resourceMngrStats rcmgr.ResourceManagerStat
	err = json.Unmarshal([]byte(exampleResourceMngrStats), &resourceMngrStats)
	if err != nil {
		panic(err)
	}

	addToExampleValues(txResponse)
	addToExampleValues(samplingStats)
	addToExampleValues(extendedHeader)
	addToExampleValues(resourceMngrStats)

	mathInt, _ := math.NewIntFromString("42")
	addToExampleValues(mathInt)

	addToExampleValues(network.Connected)
	addToExampleValues(network.ReachabilityPrivate)

	pID := protocol.ID("/celestia/mocha/ipfs/bitswap")
	addToExampleValues(pID)

	peerID := peer.ID("12D3KooWPRb5h3g9MH7sx9qfbSQZG5cXv1a2Qs3o4aW5YmmzPq82")
	addToExampleValues(peerID)

	ma, _ := multiaddr.NewMultiaddr("/ip6/::1/udp/2121/quic-v1")
	addrInfo := peer.AddrInfo{
		ID:    peerID,
		Addrs: []multiaddr.Multiaddr{ma},
	}
	addToExampleValues(addrInfo)

	namespace, err := share.NewBlobNamespaceV0([]byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0x10})
	if err != nil {
		panic(err)
	}
	addToExampleValues(namespace)

	generatedBlob, err := blob.NewBlobV0(namespace, []byte("This is an example of some blob data"))
	if err != nil {
		panic(err)
	}
	addToExampleValues(generatedBlob)

	proof := nmt.NewInclusionProof(0, 4, [][]byte{[]byte("test")}, true)
	blobProof := &blob.Proof{&proof}
	addToExampleValues(blobProof)
}

func addToExampleValues(v interface{}) {
	ExampleValues[reflect.TypeOf(v)] = v
}

func ExampleValue(t, parent reflect.Type) (interface{}, error) {
	v, ok := ExampleValues[t]
	if ok {
		return v, nil
	}

	switch t.Kind() {
	case reflect.Slice:
		out := reflect.New(t).Elem()
		val, err := ExampleValue(t.Elem(), t)
		if err != nil {
			return nil, err
		}
		out = reflect.Append(out, reflect.ValueOf(val))
		return out.Interface(), nil
	case reflect.Chan:
		return ExampleValue(t.Elem(), nil)
	case reflect.Struct:
		es, err := exampleStruct(t, parent)
		if err != nil {
			return nil, err
		}
		v := reflect.ValueOf(es).Elem().Interface()
		ExampleValues[t] = v
		return v, nil
	case reflect.Array:
		out := reflect.New(t).Elem()
		for i := 0; i < t.Len(); i++ {
			val, err := ExampleValue(t.Elem(), t)
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

func exampleStruct(t, parent reflect.Type) (interface{}, error) {
	ns := reflect.New(t)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Type == parent {
			continue
		}
		if cases.Title(language.Und, cases.NoLower).String(f.Name) == f.Name {
			val, err := ExampleValue(f.Type, t)
			if err != nil {
				return nil, err
			}
			ns.Elem().Field(i).Set(reflect.ValueOf(val))
		}
	}

	return ns.Interface(), nil
}
