package main

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	buildTime       string
	lastCommit      string
	semanticVersion string
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show information about the current binary build",
	Args:  cobra.NoArgs,
	Run:   printVersionInfo,
}

func printVersionInfo(_ *cobra.Command, _ []string) {
	fmt.Printf("Semantic version: %s\n", semanticVersion)
	fmt.Printf("System version: %s/%s\n", runtime.GOARCH, runtime.GOOS)
	fmt.Printf("Golang version: %s\n", runtime.Version())
}
