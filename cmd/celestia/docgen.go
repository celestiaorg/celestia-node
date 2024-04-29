package main

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/api/docgen"
	"github.com/celestiaorg/celestia-node/api/rpc/client"
	"github.com/celestiaorg/celestia-node/nodebuilder"
)

var docgenCmd = &cobra.Command{
	Use:   "docgen",
	Short: "docgen generates the openrpc documentation for all Celestia Node packages",
	RunE: func(_ *cobra.Command, _ []string) error {
		// 1. Collect all node's modules
		moduleNames := make([]string, 0, len(client.Modules))
		for module := range client.Modules {
			moduleNames = append(moduleNames, module)
		}

		// 2. Open the respective nodebuilder/X/service.go files for AST parsing
		nodeComments, permComments := docgen.ParseCommentsFromNodebuilderModules(moduleNames...)

		// 3. Create an OpenRPC document from the map of comments + hardcoded metadata
		doc := docgen.NewOpenRPCDocument(nodeComments, permComments)

		// 4. Register the client wrapper interface on the document
		for moduleName, module := range nodebuilder.PackageToAPI {
			doc.RegisterReceiverName(moduleName, module)
		}

		// 5. Call doc.Discover()
		d, err := doc.Discover()
		if err != nil {
			return err
		}

		// 6. Print to Stdout
		jsonOut, err := json.MarshalIndent(d, "", "    ")
		if err != nil {
			return err
		}

		_, err = os.Stdout.Write(jsonOut)
		return err
	},
}
