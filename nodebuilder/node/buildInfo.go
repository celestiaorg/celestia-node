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

const (
	emptyValue = "unknown"
)

// BuildInfo represents all necessary information about current build.
type BuildInfo struct {
	BuildTime       string `json:"build_date"`
	LastCommit      string `json:"commit"`
	SemanticVersion string `json:"semantic_version"`
	SystemVersion   string `json:"system_version"`
	GolangVersion   string `json:"golang_version"`
}

func (b *BuildInfo) GetSemanticVersion() string {
	if b.SemanticVersion == "" {
		return emptyValue
	}

	return b.SemanticVersion
}

func (b *BuildInfo) CommitShortSha() string {
	if b.LastCommit == "" {
		return emptyValue
	}

	if len(b.LastCommit) < 7 {
		return b.LastCommit
	}

	return b.LastCommit[:7]
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
