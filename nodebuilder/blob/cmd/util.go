package cmd

import (
	"encoding/json"
	"os"
)

// Define the raw content from the file input.
type blobs struct {
	Blobs []blobJSON
}

type blobJSON struct {
	Namespace string
	BlobData  string
}

func parseSubmitBlobs(path string) ([]blobJSON, error) {
	var rawBlobs blobs

	content, err := os.ReadFile(path)
	if err != nil {
		return []blobJSON{}, err
	}

	err = json.Unmarshal(content, &rawBlobs)
	if err != nil {
		return []blobJSON{}, err
	}

	return rawBlobs.Blobs, err
}
