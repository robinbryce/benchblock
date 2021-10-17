package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/robinbryce/blockbench/bbencheth/collect"
	"github.com/robinbryce/blockbench/bbencheth/load"
	"github.com/robinbryce/blockbench/bbencheth/root"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Note: https://carolynvanslyck.com/blog/2020/08/sting-of-the-viper/0
// the most succinct and clear example of cobra & viper together

var (
	envPrefix = ""
)

// Config defines the overall aggregate configuration for bbencheth.
type Config struct {
	root.Config
	Collect collect.Config
	Load    load.Config
}

func NewConfig() *Config {
	cfg := Config{}
	cfg.SetDefaults()
	return &cfg
}

func (cfg *Config) SetDefaults() {
	cfg.Config.SetDefaults()
	cfg.Collect.SetDefaults()
	cfg.Load.SetDefaults()
}

func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type RootRunner struct {
	Runnable
	namedConfigs map[string]interface{}
}

func (r RootRunner) GetNamedConfig(name string) interface{} {
	return r.namedConfigs[name]
}

func (r *RootRunner) AddOptions(vroot *viper.Viper) error {

	if err := root.SetDefaultConfig(vroot, root.GetRunnerName(r), r.cfg); err != nil {
		panic(err)
	}
	r.vroot = vroot

	f := r.cmd.PersistentFlags()
	f.SetNormalizeFunc(NormalizeOptions)

	f.StringVarP(
		&r.cfg.EthEndpoint, "eth", "e", r.cfg.EthEndpoint, `
ethereum json rpc endpoint. each thread derives a client url by adding its index
to the port (unless --singlenode is set)`)
	f.IntVar(
		&r.cfg.BasePort, "baseport", r.cfg.BasePort,
		`The first port if --eth is used. If using --staticnodes all nodes be on this port`)
	f.BoolVar(
		&r.cfg.ResolveHosts, "resolvehosts", false, "resolve target hostnames to ip addresses once at startup")
	f.IntVarP(
		&r.cfg.Retries, "retries", "c", r.cfg.Retries,
		`
number of retries for any eth request (a geometric backoff is applied between each try)`)
	f.DurationVar(
		&r.cfg.ClientTimeout, "client-timeout", r.cfg.ClientTimeout,
		"every eth request sets this timeout")
	f.BoolVar(
		&r.cfg.NoProgress, "no-progress", false,
		"disables progress meter")

	return nil
}

func (r *RootRunner) ProcessConfig() error {
	r.cfgDir = filepath.Dir(r.vroot.ConfigFileUsed())

	ReconcileOptions(r.cmd, r.vroot.Sub(root.GetRunnerName(r)))
	return nil
}

func (r *RootRunner) Run(cmd *cobra.Command, args []string) {
	fmt.Println("try one of the sub commands")
}

func NewRootCmd() *cobra.Command {

	var cmds []Runner

	rr := &RootRunner{
		Runnable: Runnable{
			name:  root.ConfigName,
			vroot: viper.New(),
			cmd: &cobra.Command{
				Use:   root.ConfigName,
				Short: "A load generator for xxx ethereum networks",
				Long: `
			Tooling for issuing transactions and capturing details of their execution`,
			},
			cfg: NewConfig(),
		},
		namedConfigs: map[string]interface{}{},
	}
	rr.namedConfigs[rr.GetName()] = rr.GetConfig()
	rr.vroot.SetConfigName(rr.GetName())
	rr.cmd.Run = rr.Run

	cmds = append(cmds, rr)
	cmds = append(cmds, NewLoaderCmd(rr, rr.cfg))
	cmds = append(cmds, NewCollectCmd(rr, rr.cfg))

	for i := 0; i < len(cmds); i++ {
		rr.namedConfigs[cmds[i].GetName()] = cmds[i].GetConfig()
		cmds[i].AddOptions(rr.vroot)
		if i == 0 {
			continue
		}
		cmds[0].GetCmd().AddCommand(cmds[i].GetCmd())
	}

	rr.GetCmd().PersistentPreRunE = func(cmd *cobra.Command, args []string) error {

		if err := SetConfigFile(rr.vroot, rr.cfgFile, rr.GetName()); err != nil {
			return err
		}

		if err := rr.vroot.ReadInConfig(); err != nil {
			// It's okay if there isn't a config file
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return err
			}
		}

		rr.vroot.SetEnvPrefix(envPrefix)
		rr.vroot.AutomaticEnv()
		for _, r := range cmds {
			r.ProcessConfig()
		}
		return nil
	}

	f := rr.GetCmd().PersistentFlags()
	f.StringVar(&rr.cfgFile, "config", "", "configuration file. all options can be set in this")

	return rr.GetCmd()
}

// ReconcileOptions merges cli options and config giving priority to the
// options.
func ReconcileOptions(cmd *cobra.Command, v *viper.Viper) {
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
			fmt.Println("from-viper", f.Name, val)
			cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
		}
	})
}

func NormalizeOptions(f *pflag.FlagSet, name string) pflag.NormalizedName {
	name = strings.Replace(name, "-", "_", -1)
	return pflag.NormalizedName(name)
}

func SetConfigFile(v *viper.Viper, cfgFile, name string) error {
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		v.AddConfigPath(".")
		v.AddConfigPath(home)
		v.SetConfigName(name)
	}
	return nil
}
