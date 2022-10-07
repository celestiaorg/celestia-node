package docgen

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	"reflect"

	go_openrpc_reflect "github.com/etclabscore/go-openrpc-reflect"
	meta_schema "github.com/open-rpc/meta-schema"

	"github.com/celestiaorg/celestia-node/das"
	"github.com/celestiaorg/celestia-node/fraud"
	"github.com/celestiaorg/celestia-node/nodebuilder/header"
	"github.com/celestiaorg/celestia-node/share/availability/light"
	"github.com/celestiaorg/celestia-node/state"
)

type Visitor struct {
	Methods map[string]ast.Node
}

func (v *Visitor) Visit(node ast.Node) ast.Visitor {
	st, ok := node.(*ast.TypeSpec)
	if !ok {
		return v
	}

	if st.Name.Name != "Module" {
		return nil
	}

	iface := st.Type.(*ast.InterfaceType)
	for _, m := range iface.Methods.List {
		if len(m.Names) > 0 {
			v.Methods[m.Names[0].Name] = m
		}
	}

	return v
}

type Comments = map[string]string

// PackageToDefaultImpl is a map that points the package name to an implementation. This is necessary for now for the
// document to be able to discover the methods. TODO: This is a temporary solution for the first prototype. The next
// version of the prototype should use mock implementations of the interfaces.
var PackageToDefaultImpl = map[string]interface{}{
	"fraud":  &fraud.ProofService{},
	"state":  &state.CoreAccessor{},
	"share":  &light.ShareAvailability{},
	"header": &header.Service{},
	"daser":  &das.DASer{},
}

func ParseCommentsFromNodebuilderModules(moduleNames ...string) Comments {
	fset := token.NewFileSet()
	nodeComments := make(Comments)
	for _, moduleName := range moduleNames {
		f, err := parser.ParseFile(fset, "nodebuilder/"+moduleName+"/service.go", nil, parser.AllErrors|parser.ParseComments)
		if err != nil {
			panic(err)
		}

		cmap := ast.NewCommentMap(fset, f, f.Comments)

		v := &Visitor{make(map[string]ast.Node)}
		ast.Walk(v, f)

		// TODO(@distractedm1nd): An issue with this could be two methods with the same name in different modules
		for mn, node := range v.Methods {
			filteredComments := cmap.Filter(node).Comments()
			if len(filteredComments) == 0 {
				nodeComments[mn] = "No comment exists yet for this method."
			} else {
				nodeComments[mn] = filteredComments[0].Text()
			}
		}
	}
	return nodeComments
}

func NewOpenRPCDocument(comments Comments) *go_openrpc_reflect.Document {
	d := &go_openrpc_reflect.Document{}

	d.WithMeta(&go_openrpc_reflect.MetaT{
		GetServersFn: func() func(listeners []net.Listener) (*meta_schema.Servers, error) {
			return func(listeners []net.Listener) (*meta_schema.Servers, error) {
				return nil, nil
			}
		},
		GetInfoFn: func() (info *meta_schema.InfoObject) {
			info = &meta_schema.InfoObject{}
			title := "Bikini Bottom API"
			info.Title = (*meta_schema.InfoObjectProperties)(&title)

			// TODO(@distractedm1nd): build time variable of celestia node version?
			version := "0.0.1"
			info.Version = (*meta_schema.InfoObjectVersion)(&version)
			return info
		},
		GetExternalDocsFn: func() (exdocs *meta_schema.ExternalDocumentationObject) {
			// TODO(@distractedm1nd): update
			return nil // FIXME
		},
	})

	appReflector := &go_openrpc_reflect.EthereumReflectorT{}

	appReflector.FnIsMethodEligible = func(m reflect.Method) bool {
		// methods are only eligible if they were found in the Module interface
		_, ok := comments[m.Name]
		if !ok {
			return false
		}

		// TODO(@distractedm1nd): find out why chans are excluded in lotus. is this a must?
		for i := 0; i < m.Func.Type().NumOut(); i++ {
			if m.Func.Type().Out(i).Kind() == reflect.Chan {
				return false
			}
		}
		return go_openrpc_reflect.EthereumReflector.IsMethodEligible(m)
	}

	appReflector.FnGetMethodName = func(
		moduleName string,
		r reflect.Value,
		m reflect.Method,
		funcDecl *ast.FuncDecl,
	) (string, error) {
		return moduleName + "." + m.Name, nil
	}

	appReflector.FnGetMethodSummary = func(r reflect.Value, m reflect.Method, funcDecl *ast.FuncDecl) (string, error) {
		if v, ok := comments[m.Name]; ok {
			return v, nil
		}
		return "", nil // noComment
	}

	d.WithReflector(appReflector)
	return d
}
