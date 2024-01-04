package utils

import "strconv"

// DO NOT MERGE: deliberately broken
// reverse function to show fuzz workflow
func DummyReverse(s string) string {
	b := []byte(s)
	for i, j := 0, len(b)-1; i < len(b)/2; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return string(b)
}

// ProcessNumber checks if the byte slice can be converted to an integer.
func DummyProcessNumber(data []byte) bool {
	_, err := strconv.Atoi(string(data))
	return err == nil
}
