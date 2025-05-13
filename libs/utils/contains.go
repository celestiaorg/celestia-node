package utils

import (
	"golang.org/x/exp/slices"
)

// ContainsWildcard checks if a slice contains the wildcard "*"
func ContainsWildcard(slice []string) bool {
	return slices.Contains(slice, "*")
}
