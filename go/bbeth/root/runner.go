package root

import (
	"encoding/json"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type Runner interface {
	GetName() string
	GetParent() Runner
	GetCmd() *cobra.Command
	GetConfig() interface{}
	// GenNamedConfig returns the config for the base config name (base config name does not include parents)
	GetNamedConfig(string) interface{}
	AddOptions(*viper.Viper) error
	ProcessConfig() error
}

// SetDefaultConfig is a convenience for pushing defaults from a struct instance
// into viper
func SetDefaultConfig(v *viper.Viper, name string, cfg interface{}) error {
	marshaled, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	var mapped map[string]interface{}
	if err := json.Unmarshal(marshaled, &mapped); err != nil {
		return err
	}
	v.SetDefault(name, mapped)
	return nil
}

// GetRunnerName returns the name of the runner in the Viper config. Which is
// the '.' delimited catenation of its name with all its parents.
func GetRunnerName(r Runner) string {

	ss := []string{r.GetName()}

	for p := r.GetParent(); p != nil; p = p.GetParent() {
		ss = append(ss, p.GetName())
	}

	// now reverse it
	last := len(ss) - 1
	for i := 0; i < len(ss)/2; i++ {
		ss[i], ss[last-i] = ss[last-i], ss[i]
	}
	return strings.Join(ss, ".")
}
