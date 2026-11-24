package docgen_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/api/docgen"
	"github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/nodebuilder"
)

func TestOpenRPCAuthenticationExamples(t *testing.T) {
	t.Chdir("../..")
	moduleNames := make([]string, 0, len(client.Modules))
	for name := range client.Modules {
		moduleNames = append(moduleNames, name)
	}
	comments, permissions := docgen.ParseCommentsFromNodebuilderModules(moduleNames...)
	doc := docgen.NewOpenRPCDocument(comments, permissions)
	for name, api := range nodebuilder.PackageToAPI {
		doc.RegisterReceiverName(name, api)
	}
	spec, err := doc.Discover()
	require.NoError(t, err)
	require.NotEmpty(t, *spec.Methods)
	for _, method := range *spec.Methods {
		t.Run(string(*method.Name), func(t *testing.T) {
			moduleName, methodName, ok := strings.Cut(string(*method.Name), ".")
			require.True(t, ok)
			field, ok := reflect.TypeOf(client.Modules[moduleName]).Elem().FieldByName(methodName)
			require.True(t, ok)
			permission := field.Tag.Get("perm")
			require.NotEmpty(t, permission)
			require.NotNil(t, method.Description)
			description := string(*method.Description)
			require.Contains(t, description, "Auth level: "+permission)
			require.Contains(t, description, "celestia <node-type> auth "+permission)
			require.Contains(t, description, "--p2p.network")
			require.Contains(t, description, "--node.store")
			require.Contains(t, description, "Authorization: Bearer <token>")
			switch string(*method.Name) {
			case "header.Subscribe", "blob.Subscribe", "blob.SubscribeFromStartHeight":
				require.Contains(t, description, "requires WebSockets")
				require.Contains(t, description, "ws://localhost:26658")
				require.Contains(t, description, "HTTP upgrade request")
				require.NotContains(t, description, "curl")
				require.NotContains(t, description, "--data-binary")
				return
			}
			require.Contains(t, description, "request.json")
			require.Contains(t, description, `-H "Authorization: Bearer $CELESTIA_NODE_AUTH_TOKEN"`)
			require.Contains(t, description, `-H "Content-Type: application/json"`)
			require.Contains(t, description, "--data-binary @request.json")
			require.Contains(t, description, "http://localhost:26658")
		})
	}
}
