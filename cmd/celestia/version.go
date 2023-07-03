package main

import (
	"fmt"
	"runtime"

	_ "embed"

	"github.com/spf13/cobra"
)

var (
	buildTime string
	//go:embed lastCommit.txt
	lastCommit string
	//go:embed semanticVersion.txt
	semanticVersion string

	systemVersion = fmt.Sprintf("%s/%s", runtime.GOARCH, runtime.GOOS)
	golangVersion = runtime.Version()
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show information about the current binary build",
	Args:  cobra.NoArgs,
	Run:   printBuildInfo,
}

func printBuildInfo(_ *cobra.Command, _ []string) {
	fmt.Printf("Semantic version: %s\n", semanticVersion)
	fmt.Printf("Commit: %s\n", lastCommit)
	fmt.Printf("Build Date: %s\n", buildTime)
	fmt.Printf("System version: %s\n", systemVersion)
	fmt.Printf("Golang version: %s\n", golangVersion)
}
