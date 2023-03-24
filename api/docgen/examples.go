package docgen

import (
	"fmt"
	"reflect"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/celestiaorg/celestia-node/share/eds/byzantine"
)

var ExampleValues = map[reflect.Type]interface{}{
	reflect.TypeOf(""):         "string value",
	reflect.TypeOf(uint64(42)): uint64(42),
	reflect.TypeOf(uint32(42)): uint32(42),
	// reflect.TypeOf(int32(42)):   int32(42),
	reflect.TypeOf(int64(42)):             int64(42),
	reflect.TypeOf(byte(7)):               byte(7),
	reflect.TypeOf(float64(42)):           float64(42),
	reflect.TypeOf(true):                  true,
	reflect.TypeOf([]byte{}):              []byte("byte array"),
	reflect.TypeOf(byzantine.BadEncoding): byzantine.BadEncoding,
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
