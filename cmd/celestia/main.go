package main

import (
	"context"
	"os"

	"github.com/celestiaorg/celestia-node/cmd"
)

func main() {
	err := run()
	if err != nil {
		os.Exit(1)
	}
}

func run() error {
	return rootCmd.ExecuteContext(cmd.WithEnv(context.Background()))
}

var rootCmd = cmd.NewRootCmd()
