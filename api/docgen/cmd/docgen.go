package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/celestiaorg/celestia-node/api/docgen"
)

func main() {
	// 1. Parse the module names from the command line arguments
	var moduleNames = os.Args[1:]

	// 2. Open the respective nodebuilder/X/service.go files for AST parsing
	nodeComments := docgen.ParseCommentsFromNodebuilderModules(moduleNames...)

	// 3. Create an OpenRPC document from the map of comments + hardcoded metadata
	doc := docgen.NewOpenRPCDocument(nodeComments)

	// 4. Register the client wrapper interface on the document
	for moduleName, module := range docgen.PackageToDefaultImpl {
		doc.RegisterReceiverName(moduleName, module)
	}

	// 5. Call doc.Discover()
	d, err := doc.Discover()
	if err != nil {
		panic(err)
	}

	// 6. Print to Stdout
	jsonOut, err := json.MarshalIndent(d, "", "    ")
	if err != nil {
		log.Fatalln(err)
	}
	if err != nil {
		panic(err)
	}

	_, err = os.Stdout.Write(jsonOut)
	if err != nil {
		panic(err)
	}
}
