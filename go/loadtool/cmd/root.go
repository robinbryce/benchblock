package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/robinbryce/blockbench/loadtool/loader"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Note: https://carolynvanslyck.com/blog/2020/08/sting-of-the-viper/0
// the most succinct and clear example of cobra & viper together

var (
	envPrefix = "ETHLOAD"
)

func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func NewRootCmd() *cobra.Command {

	var (
		cfgFile string
		cfg     loader.Config
	)
	rootCmd := &cobra.Command{
		Use:   "ethload",
		Short: "A load generator for xxx ethereum networks",
		Long: `
Uses the native go-ethereum libaries to deploy the idiomatic get/set/add
contract and invoke state changing functions to generate transaction load for
ethereum networks`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// You can bind cobra and viper in a few locations, but PersistencePreRunE on the root command works well
			return ProcessConfig(cmd, cfgFile)
		},

		Run: func(cmd *cobra.Command, args []string) {

			a, err := loader.NewAdder(context.Background(), &cfg)
			cobra.CheckErr(err)
			if cfg.RunOne {
				err = a.RunOne()
				cobra.CheckErr(err)
				return
			}
			a.Run()
		},
	}

	f := rootCmd.PersistentFlags()
	f.SetNormalizeFunc(normalize)
	f.StringVar(&cfgFile, "config", "", "configuration file. all options can be set in this")
	loader.AddOptions(rootCmd, &cfg)

	return rootCmd
}

func ProcessConfig(cmd *cobra.Command, cfgFile string) error {

	v := viper.New()
	v.SetConfigName("ethload")

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		v.AddConfigPath(".")
		v.AddConfigPath(home)
		v.SetConfigName("ethload")
	}
	loader.SetViperDefaults(v)
	if err := v.ReadInConfig(); err != nil {
		// It's okay if there isn't a config file
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	v.SetEnvPrefix(envPrefix)
	v.AutomaticEnv()

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
	return nil
}

func normalize(f *pflag.FlagSet, name string) pflag.NormalizedName {
	name = strings.Replace(name, "-", "_", -1)
	return pflag.NormalizedName(name)
}
