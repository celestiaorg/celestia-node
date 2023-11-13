package utils_fuzz

import (
	"fmt"
	"testing"

	"github.com/celestiaorg/celestia-node/libs/utils"
)

// FuzzProcessNumber tests the ProcessNumber function.
func FuzzProcessNumber(f *testing.F) {
	// Seed the corpus with a valid number
	f.Add([]byte("42"))
	f.Add([]byte("42!!!!!"))
	fmt.Println("process number")

	f.Fuzz(func(t *testing.T, data []byte) {
		// This fuzz test will never fail since we're not asserting any conditions.
		// It just processes the input data.
		result := utils.DummyProcessNumber(data)
		if !result {
			f.Failed()
		}
	})
}
