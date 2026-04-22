package blob

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	libshare "github.com/celestiaorg/go-square/v3/share"
)

func FuzzBlobUnmarshal(f *testing.F) {
	if testing.Short() {
		f.Skip("in -short mode")
	}

	// 1. Generate the corpus.
	libBlobs, err := libshare.GenerateV0Blobs([]int{16}, false)
	if err != nil {
		f.Fatal(err)
	}

	blobs, err := convertBlobs(libBlobs...)
	if err != nil {
		f.Fatal(err)
	}

	for _, blob := range blobs {
		jsonBlob, err := blob.MarshalJSON()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(jsonBlob)
	}

	// 2. Run the fuzzer.
	f.Fuzz(func(t *testing.T, jsonBlob []byte) {
		blob := new(Blob)
		_ = blob.UnmarshalJSON(jsonBlob)
	})
}

type verifyCorpus struct {
	CP         *CommitmentProof `json:"commitment_proof"`
	Root       []byte           `json:"root"`
	Commitment Commitment       `json:"commitment"`
}

func FuzzCommitmentProofVerify(f *testing.F) {
	if testing.Short() {
		f.Skip("in -short mode")
	}

	path := filepath.Join("testdata", "fuzz-corpus", "verify-*.json")
	paths, err := filepath.Glob(path)
	if err != nil {
		f.Fatal(err)
	}

	// Add the corpra.
	for _, path := range paths {
		blob, err := os.ReadFile(path)
		if err != nil {
			f.Fatal(err)
		}

		var values []*verifyCorpus
		if err := json.Unmarshal(blob, &values); err != nil {
			f.Fatal(err)
		}

		for _, value := range values {
			blob, err := json.Marshal(value)
			if err != nil {
				f.Fatal(err)
			}
			f.Add(blob)
		}
	}

	f.Fuzz(func(t *testing.T, valueJSON []byte) {
		val := new(verifyCorpus)
		if err := json.Unmarshal(valueJSON, val); err != nil {
			return
		}
		commitProof := val.CP
		if commitProof == nil {
			return
		}
		_ = commitProof.Verify(val.Root, val.Commitment)
	})
}
