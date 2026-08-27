package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/celestiaorg/celestia-node/nodebuilder/node"
)

const (
	outputFlag = "output"

	outputText = "text"
	outputJSON = "json"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show information about the current binary build",
	Args:  cobra.NoArgs,
	RunE:  printBuildInfo,
}

func init() {
	versionCmd.Flags().StringP(outputFlag, "o", outputText, "output format of the build information: text or json")
}

func printBuildInfo(cmd *cobra.Command, _ []string) error {
	output, err := cmd.Flags().GetString(outputFlag)
	if err != nil {
		return err
	}

	buildInfo := node.GetBuildInfo()

	switch output {
	case outputJSON:
		bytes, err := json.MarshalIndent(buildInfo, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(bytes))
	case outputText:
		fmt.Printf("Semantic version: %s\n", buildInfo.SemanticVersion)
		fmt.Printf("Commit: %s\n", buildInfo.LastCommit)
		fmt.Printf("Build Date: %s\n", buildInfo.BuildTime)
		fmt.Printf("System version: %s\n", buildInfo.SystemVersion)
		fmt.Printf("Golang version: %s\n", buildInfo.GolangVersion)
	default:
		return fmt.Errorf("invalid --%s value %q: must be one of [%s, %s]", outputFlag, output, outputText, outputJSON)
	}

	return nil
}
