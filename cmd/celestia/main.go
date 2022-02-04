package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/celestiaorg/celestia-node/cmd"
)

const (
	defaultConfigFileName = "config"
	envPrefix             = "STING"
)

func init() {
	rootCmd.AddCommand(
		bridgeCmd,
		lightCmd,
		versionCmd,
	)
	rootCmd.SetHelpCommand(&cobra.Command{})
}

func main() {
	err := run()
	if err != nil {
		os.Exit(1)
	}
}

func run() error {
	return rootCmd.ExecuteContext(cmd.WithEnv(context.Background()))
}

var rootCmd = &cobra.Command{
	Use: "celestia [  bridge  ||  light  ] [subcommand]",
	Short: `
	  / ____/__  / /__  _____/ /_(_)___ _
	 / /   / _ \/ / _ \/ ___/ __/ / __  /
	/ /___/  __/ /  __(__  ) /_/ / /_/ /
	\____/\___/_/\___/____/\__/_/\__,_/
	`,
	Args: cobra.NoArgs,
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true,
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initializeConfig(cmd)
	},
}

func initializeConfig(cmd *cobra.Command) error {
	v := viper.New()

	v.SetConfigName(defaultConfigFileName)

	v.AddConfigPath(".celestia-node")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	v.SetEnvPrefix(envPrefix)

	v.AutomaticEnv()

	bindFlags(cmd, v)

	return nil
}

func bindFlags(cmd *cobra.Command, v *viper.Viper) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Environment variables can't have dashes in them, so bind them to their equivalent
		// keys with underscores, e.g. --favorite-color to STING_FAVORITE_COLOR
		if strings.Contains(f.Name, "-") {
			envVarSuffix := strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
			v.BindEnv(f.Name, fmt.Sprintf("%s_%s", envPrefix, envVarSuffix))
		}

		// Apply the viper config value to the flag when the flag is not set and viper has a value
		if !f.Changed && v.IsSet(f.Name) {
			val := v.Get(f.Name)
			cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
		}
	})
}
