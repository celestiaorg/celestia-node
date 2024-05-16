//go:build conformance

package api

import (
	"reflect"
	"testing"

	blobA "github.com/celestiaorg/celestia-openrpc/types/blob"
	daA "github.com/celestiaorg/celestia-openrpc/types/da"
	dasA "github.com/celestiaorg/celestia-openrpc/types/das"
	fraudA "github.com/celestiaorg/celestia-openrpc/types/fraud"
	headerA "github.com/celestiaorg/celestia-openrpc/types/header"
	nodeA "github.com/celestiaorg/celestia-openrpc/types/node"
	p2pA "github.com/celestiaorg/celestia-openrpc/types/p2p"
	shareA "github.com/celestiaorg/celestia-openrpc/types/share"
	stateA "github.com/celestiaorg/celestia-openrpc/types/state"

	blob "github.com/celestiaorg/celestia-node/nodebuilder/blob"
	da "github.com/celestiaorg/celestia-node/nodebuilder/da"
	das "github.com/celestiaorg/celestia-node/nodebuilder/das"
	fraud "github.com/celestiaorg/celestia-node/nodebuilder/fraud"
	header "github.com/celestiaorg/celestia-node/nodebuilder/header"
	node "github.com/celestiaorg/celestia-node/nodebuilder/node"
	p2p "github.com/celestiaorg/celestia-node/nodebuilder/p2p"
	share "github.com/celestiaorg/celestia-node/nodebuilder/share"
	state "github.com/celestiaorg/celestia-node/nodebuilder/state"
)

// TestAPIEquivalence tests that the API structs in celestia-openrpc and nodebuilder are equivalent.
func TestAPIEquivalence(t *testing.T) {
	libraryStructs := []interface{}{
		blobA.API{},
		daA.API{},
		dasA.API{},
		fraudA.API{},
		headerA.API{},
		nodeA.API{},
		p2pA.API{},
		shareA.API{},
		stateA.API{},
	}

	apiStructs := []interface{}{
		blob.API{}.Internal,
		da.API{}.Internal,
		das.API{}.Internal,
		fraud.API{}.Internal,
		header.API{}.Internal,
		node.API{}.Internal,
		p2p.API{}.Internal,
		share.API{}.Internal,
		state.API{}.Internal,
	}

	moduleNames := []string{
		"blob",
		"da",
		"das",
		"fraud",
		"header",
		"node",
		"p2p",
		"share",
		"state",
	}

	for i := 0; i < len(libraryStructs); i++ {
		compareStructs(t, libraryStructs[i], apiStructs[i], moduleNames[i])
	}
}

func compareStructs(t *testing.T, structA, structB interface{}, moduleName string) {
	typeA := reflect.TypeOf(structA)
	typeB := reflect.TypeOf(structB)

	if typeA.NumField() != typeB.NumField() {
		t.Fatalf("%s API has a comflicting number of fields: %d != %d", moduleName, typeA.NumField(), typeB.NumField())
	}

	fieldsA := map[string]reflect.StructField{}
	fieldsB := map[string]reflect.StructField{}

	for i := 0; i < typeA.NumField(); i++ {
		fieldsA[typeA.Field(i).Name] = typeA.Field(i)
	}
	for i := 0; i < typeB.NumField(); i++ {
		fieldsB[typeB.Field(i).Name] = typeB.Field(i)
	}

	for name, fieldA := range fieldsA {
		fieldB, exists := fieldsB[name]
		if !exists {
			t.Errorf("Field %s exists in struct %s but not in struct %s", name, typeA.Name(), typeB.Name())
			continue
		}

		if fieldA.Name != fieldB.Name {
			t.Errorf("%s API: method name mismatch: %s != %s", moduleName, fieldA.Name, fieldB.Name)
		}

		funcA := fieldA.Type
		funcB := fieldB.Type

		if funcA.Kind() != reflect.Func || funcB.Kind() != reflect.Func {
			t.Errorf("%s API: Field %s is not a function: %s, %s", moduleName, name, funcA.Kind(), funcB.Kind())
			continue
		}

		if funcA.NumIn() != funcB.NumIn() {
			t.Errorf("%s API: Field %s parameter count mismatch: %d != %d", moduleName, name, funcA.NumIn(), funcB.NumIn())
		}

		if funcA.NumOut() != funcB.NumOut() {
			t.Errorf("%s API: Field %s return value count mismatch: %d != %d", moduleName, name, funcA.NumOut(), funcB.NumOut())
		}

		if fieldA.Tag != fieldB.Tag {
			t.Errorf("%s API: Field %s tag mismatch: %s != %s", moduleName, name, fieldA.Tag, fieldB.Tag)
		}
	}
}
