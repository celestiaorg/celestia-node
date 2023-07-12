package version

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	buildTime       string
	lastCommit      string
	semanticVersion string

	systemVersion = fmt.Sprintf("%s/%s", runtime.GOARCH, runtime.GOOS)
	golangVersion = runtime.Version()
)

var Cmd = &cobra.Command{
	Use:   "version",
	Short: "Show information about the current binary build",
	Args:  cobra.NoArgs,
	Run:   printBuildInfo,
}

// BuildInfo represents all necessary information about current build.
type BuildInfo struct {
	BuildTime       string
	LastCommit      string
	SemanticVersion string
	SystemVersion   string
	GolandVersion   string
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

func printBuildInfo(_ *cobra.Command, _ []string) {
	fmt.Printf("Semantic version: %s\n", semanticVersion)
	fmt.Printf("Commit: %s\n", lastCommit)
	fmt.Printf("Build Date: %s\n", buildTime)
	fmt.Printf("System version: %s\n", systemVersion)
	fmt.Printf("Golang version: %s\n", golangVersion)
}
