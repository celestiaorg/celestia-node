//go:build fuzz

package reverse

// func FuzzReverse(f *testing.F) {
// 	tests := []string{"Hello, world", " ", "!12345"}
// 	for _, tc := range tests {
// 		f.Add(tc) // Use f.Add to provide a seed corpus
// 	}
// 	f.Fuzz(func(t *testing.T, orig string) {
// 		rev := Reverse(orig)
// 		doubleRev := Reverse(rev)
// 		if orig != doubleRev {
// 			t.Errorf("Before: %q, after: %q", orig, doubleRev)
// 		}
// 		if utf8.ValidString(orig) && !utf8.ValidString(rev) {
// 			t.Errorf("Reverse produced invalid UTF-8 string %q", rev)
// 		}
// 	})
// }
