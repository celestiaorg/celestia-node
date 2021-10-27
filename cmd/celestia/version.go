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

func init() {
	versionCmd.AddCommand()
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show information about the current binary build",
	Args:  cobra.NoArgs,
	Run:   getBuildInfo,
}

func getBuildInfo(cmd *cobra.Command, args []string) {
	fmt.Printf("Semantic version: %s\n", semanticVersion)
	fmt.Printf("Commit: %s\n", lastCommit)
	fmt.Printf("Build Date: %s\n", buildTime)
	fmt.Printf("System version: %s/%s\n", runtime.GOARCH, runtime.GOOS)
	fmt.Printf("Golang version: %s\n", runtime.Version())
}
