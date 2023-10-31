package reverse

import "strconv"

// ProcessNumber checks if the byte slice can be converted to an integer.
func ProcessNumber(data []byte) bool {
	_, err := strconv.Atoi(string(data))
	return err == nil
}
