package docker

import (
	"testing"

	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
)

func TestOfficialDigestOnlyForTheOfficialRepository(t *testing.T) {
	official := model.OfficialImageRepository + "@sha256:" + hex64('b')
	digests := []string{"example.invalid/other@sha256:" + hex64('a'), official}
	for ref, want := range map[string]string{
		model.OfficialImageRepository + ":v0.33.0": official,
		official:                      official,
		model.OfficialImageRepository: official,
		"localhost:5000/celestiaorg/celestia-node:v0.33.0": "",
		"celestia-node:canary":                             "",
		"example.invalid/other:latest":                     "",
		model.OfficialImageRepository + "-extra:v0.33.0":   "",
	} {
		if got := officialDigest(ref, digests); got != want {
			t.Errorf("officialDigest(%q) = %q, want %q", ref, got, want)
		}
	}
	// A local build records a digest under its own name only.
	if got := officialDigest("cnc-local-test", []string{"cnc-local-test@sha256:" + hex64('c')}); got != "" {
		t.Errorf("local build resolved to registry digest %q", got)
	}
}

func hex64(c byte) string {
	b := make([]byte, 64)
	for i := range b {
		b[i] = c
	}
	return string(b)
}
