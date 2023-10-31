//go:build fuzz

package reverse

import (
	"fmt"
	"testing"
)

// FuzzProcessNumber tests the ProcessNumber function.
func FuzzProcessNumber(f *testing.F) {
	// Seed the corpus with a valid number
	f.Add([]byte("42"))
	fmt.Println("process number")

	f.Fuzz(func(t *testing.T, data []byte) {
		// This fuzz test will never fail since we're not asserting any conditions.
		// It just processes the input data.
		ProcessNumber(data)
	})
}

// FuzzProcessNumber tests the ProcessNumber function.
func FuzzOtherTHing(f *testing.F) {
	// Seed the corpus with a valid number
	f.Add([]byte("42"))
	fmt.Println("other thing")

	f.Fuzz(func(t *testing.T, data []byte) {
		// This fuzz test will never fail since we're not asserting any conditions.
		// It just processes the input data.
		ProcessNumber(data)
	})
}
