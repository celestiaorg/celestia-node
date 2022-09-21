package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	buildTime  string
	lastCommit string
)

var buildInfo = &cobra.Command{
	Use:   "build",
	Short: "Show information about the current binary build",
	Args:  cobra.NoArgs,
	Run:   printBuildInfo,
}

func printBuildInfo(_ *cobra.Command, _ []string) {
	fmt.Printf("Commit: %s\n", lastCommit)
	fmt.Printf("Build Date: %s\n", buildTime)
}
