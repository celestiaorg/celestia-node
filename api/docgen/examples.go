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

//go:embed "exampledata/extendedHeader.json"
var exampleExtendedHeader string

//go:embed "exampledata/samplingStats.json"
var exampleSamplingStats string

//go:embed "exampledata/txResponse.json"
var exampleTxResponse string

//go:embed "exampledata/resourceManagerStats.json"
var exampleResourceMngrStats string

//go:embed "exampledata/blob.json"
var exampleBlob string

//go:embed "exampledata/blobProof.json"
var exampleBlobProof string

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
	reflect.TypeOf(time.Duration(0)):         time.Second,
	reflect.TypeOf(node.Full):                node.Full,
	reflect.TypeOf(auth.Permission("admin")): auth.Permission("admin"),
	reflect.TypeOf(byzantine.BadEncoding):    byzantine.BadEncoding,
	reflect.TypeOf((*fraud.Proof[*header.ExtendedHeader])(nil)).Elem(): byzantine.CreateBadEncodingProof(
		[]byte("bad encoding proof"),
		42,
		&byzantine.ErrByzantine{
			Index:  0,
			Axis:   rsmt2d.Axis(0),
			Shares: []*byzantine.ShareWithProof{},
		},
	),
	reflect.TypeOf((*error)(nil)).Elem(): errors.New("error"),
	reflect.TypeOf(state.Balance{}):      state.Balance{Amount: sdk.NewInt(42), Denom: "utia"},
}

func init() {
	addToExampleValues(share.EmptyEDS())
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

	var exBlob *blob.Blob
	err = json.Unmarshal([]byte(exampleBlob), &exBlob)
	if err != nil {
		panic(err)
	}

	var blobProof *blob.Proof
	err = json.Unmarshal([]byte(exampleBlobProof), &blobProof)
	if err != nil {
		panic(err)
	}

	addToExampleValues(exBlob)
	addToExampleValues(exBlob.Blob)
	addToExampleValues(blobProof)
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

	commitment, err := base64.StdEncoding.DecodeString("aHlbp+J9yub6hw/uhK6dP8hBLR2mFy78XNRRdLf2794=")
	if err != nil {
		panic(err)
	}
	addToExampleValues(blob.Commitment(commitment))

	// randomly generated namespace that's used in the blob example above
	// (AAAAAAAAAAAAAAAAAAAAAAAAAAAAAMJ/xGlNMdE=)
	namespace, err := libshare.NewV0Namespace([]byte{0xc2, 0x7f, 0xc4, 0x69, 0x4d, 0x31, 0xd1})
	if err != nil {
		panic(err)
	}
	addToExampleValues(namespace)

	hashStr := "453D0BC3CB88A2ED6F2E06021383B22C72D25D7741AE51B4CAE1AD34D72A3F07"
	hash, err := hex.DecodeString(hashStr)
	if err != nil {
		panic(err)
	}
	addToExampleValues(libhead.Hash(hash))

	txConfig := state.NewTxConfig(
		state.WithGasPrice(0.002),
		state.WithGas(142225),
		state.WithKeyName("my_celes_key"),
		state.WithSignerAddress("celestia1pjcmwj8w6hyr2c4wehakc5g8cfs36aysgucx66"),
		state.WithFeeGranterAddress("celestia1hakc56ax66ypjcmwj8w6hyr2c4g8cfs3wesguc"),
	)
	addToExampleValues(txConfig)

	addToExampleValues(rsmt2d.Row)
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
