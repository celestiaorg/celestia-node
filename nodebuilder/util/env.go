package util

import (
	"os"
)

var (
	IsBootstrapper = getEnv("BOOTSTRAPPER", "false") == "true"
)

// Get environment variable or fallback to default value
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
