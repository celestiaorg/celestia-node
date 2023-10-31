package keystore

import (
	"testing"
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
		k := KeyName(data)
		encoded := k.Base32()
		if _, err := KeyNameFromBase32(encoded); err != nil {
			t.Errorf("error decoding base32: %v", err)
		}
	})
}
