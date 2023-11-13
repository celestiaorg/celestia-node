package keystore_fuzz

import (
	"testing"

	"github.com/celestiaorg/celestia-node/libs/keystore"
)

func FuzzKeyStoreName(f *testing.F) {
	corpus := []string{
		"test",
		"test2",
		"test3",
		">F?FD?FDSJFKL$&*(#W)",
	}

	for _, c := range corpus {
		f.Add(c)
	}

	f.Fuzz(func(t *testing.T, data string) {
		k := keystore.KeyName(data)
		encoded := k.Base32()
		if _, err := keystore.KeyNameFromBase32(encoded); err != nil {
			t.Errorf("error decoding base32: %v", err)
		}
	})
}
