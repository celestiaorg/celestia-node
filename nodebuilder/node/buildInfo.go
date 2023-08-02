package node

import (
	"fmt"
	"runtime"
)

var (
	buildTime       string
	lastCommit      string
	semanticVersion string

	systemVersion = fmt.Sprintf("%s/%s", runtime.GOARCH, runtime.GOOS)
	golangVersion = runtime.Version()
)

// BuildInfo represents all necessary information about current build.
type BuildInfo struct {
	BuildTime       string
	LastCommit      string
	SemanticVersion string
	SystemVersion   string
	GolangVersion   string
}

// GetBuildInfo returns information about current build.
func GetBuildInfo() *BuildInfo {
	return &BuildInfo{
		buildTime,
		lastCommit,
		semanticVersion,
		systemVersion,
		golangVersion,
	}
}
