package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/api/docgen"
	"github.com/celestiaorg/celestia-node/nodebuilder"
)

var rootCmd = &cobra.Command{
	Use:   "docgen [packages]",
	Short: "docgen generates the openrpc documentation for Celestia Node packages",
	RunE: func(cmd *cobra.Command, moduleNames []string) error {
		// 1. Open the respective nodebuilder/X/service.go files for AST parsing
		nodeComments, permComments := docgen.ParseCommentsFromNodebuilderModules(moduleNames...)

		// 2. Create an OpenRPC document from the map of comments + hardcoded metadata
		doc := docgen.NewOpenRPCDocument(nodeComments, permComments)

		// 3. Register the client wrapper interface on the document
		for moduleName, module := range nodebuilder.PackageToAPI {
			doc.RegisterReceiverName(moduleName, module)
		}

		// 4. Call doc.Discover()
		d, err := doc.Discover()
		if err != nil {
			return err
		}

		// 5. Print to Stdout
		jsonOut, err := json.MarshalIndent(d, "", "    ")
		if err != nil {
			return err
		}

		_, err = os.Stdout.Write(jsonOut)
		return err
	},
}

func main() {
	err := run()
	if err != nil {
		os.Exit(1)
	}
}

func run() error {
	return rootCmd.ExecuteContext(context.Background())
}
