package docgen

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"reflect"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/header"
	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
	"github.com/celestiaorg/celestia-node/state"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/rsmt2d"
)

//go:embed "exampledata/extendedHeader.json"
var exampleExtendedHeader string

//go:embed "exampledata/samplingStats.json"
var exampleSamplingStats string

//go:embed "exampledata/txResponse.json"
var exampleTxResponse string

var ExampleValues = map[reflect.Type]interface{}{
	reflect.TypeOf(""):                "string value",
	reflect.TypeOf(uint64(42)):        uint64(42),
	reflect.TypeOf(uint32(42)):        uint32(42),
	reflect.TypeOf(int32(42)):         int32(42),
	reflect.TypeOf(int64(42)):         int64(42),
	reflect.TypeOf(byte(7)):           byte(7),
	reflect.TypeOf(float64(42)):       float64(42),
	reflect.TypeOf(true):              true,
	reflect.TypeOf([]byte{}):          []byte("byte array"),
	reflect.TypeOf(fraud.BadEncoding): fraud.BadEncoding,
	reflect.TypeOf((*fraud.Proof)(nil)).Elem(): byzantine.CreateBadEncodingProof(
		[]byte("bad encoding proof"),
		42,
		&byzantine.ErrByzantine{
			Index:  0,
			Axis:   rsmt2d.Axis(0),
			Shares: []*byzantine.ShareWithProof{},
		},
	),
}

func init() {
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount(app.Bech32PrefixAccAddr, app.Bech32PrefixAccPub)
	cfg.SetBech32PrefixForValidator(app.Bech32PrefixValAddr, app.Bech32PrefixValPub)
	cfg.Seal()

	fraudProof := byzantine.CreateBadEncodingProof(
		[]byte("bad encoding proof"),
		42,
		&byzantine.ErrByzantine{
			Index:  0,
			Axis:   rsmt2d.Axis(0),
			Shares: []*byzantine.ShareWithProof{},
		},
	)
	// this trick lets us use an interface type as a key in the map
	ExampleValues[reflect.TypeOf((*fraud.Proof)(nil)).Elem()] = fraudProof

	var txResponse *state.TxResponse
	err := json.Unmarshal([]byte(exampleTxResponse), &txResponse)
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

	addToExampleValues(txResponse)
	addToExampleValues(samplingStats)
	addToExampleValues(extendedHeader)

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

	mathInt, _ := math.NewIntFromString("42")
	addToExampleValues(mathInt)
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
