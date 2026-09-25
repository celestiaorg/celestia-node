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
	// An image pulled by index digest can also list a platform digest; the
	// requested one is kept.
	platform := model.OfficialImageRepository + "@sha256:" + hex64('d')
	if got := officialDigest(official, []string{platform, official}); got != official {
		t.Errorf("requested digest %q resolved to %q", official, got)
	}
	// A reference with both a tag and a digest keeps the digest it names.
	tagged := model.OfficialImageRepository + ":v0.33.0@sha256:" + hex64('b')
	if got := officialDigest(tagged, []string{platform, official}); got != official {
		t.Errorf("tagged digest reference %q resolved to %q", tagged, got)
	}
	// A requested digest that Docker did not record is not swapped for another.
	if got := officialDigest(official, []string{platform}); got != "" {
		t.Errorf("unrecorded digest %q resolved to %q", official, got)
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
