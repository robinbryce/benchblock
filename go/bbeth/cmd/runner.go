package cmd

import (
	"github.com/robinbryce/benchblock/bbencheth/root"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// command runner things
type Runner interface {
	root.Runner
}

type Runnable struct {
	parent  Runner
	name    string
	cmd     *cobra.Command
	vroot   *viper.Viper
	cfg     *Config
	cfgFile string
	cfgDir  string
}

func (r Runnable) GetName() string        { return r.name }
func (r Runnable) GetCmd() *cobra.Command { return r.cmd }
func (r Runnable) GetParent() root.Runner { return r.parent }
func (r Runnable) GetConfig() interface{} { return &r.cfg.Config }

func (r *Runnable) GetNamedConfig(name string) interface{} {
	rroot := GetRootRunner(r)
	if rroot == nil {
		return nil
	}
	return rroot.GetNamedConfig(name)
}
func (r *Runnable) AddOptions(v *viper.Viper) error { return nil }
func (r *Runnable) ProcessConfig() error            { return nil }

func GetRootRunner(r Runner) Runner {

	if r == nil {
		return nil
	}

	for parent := r.GetParent(); parent != nil; r, parent = parent, parent.GetParent() {
	}

	return r
}
