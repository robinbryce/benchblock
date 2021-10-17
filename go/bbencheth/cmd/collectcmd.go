package cmd

// This command collects the blocks only. It is provided to facilitate in
// cluster testing using Kubernetes Jobs. It is awkward to retrieve results
// data from k8s jobs so we run the load in cluster and collect the results
// directly from the client (eg the git hub action context)

import (
	"context"
	"path/filepath"

	"github.com/robinbryce/blockbench/bbencheth/collect"
	"github.com/robinbryce/blockbench/bbencheth/root"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type CollectRunner struct {
	Runnable
}

func (r *CollectRunner) GetConfig() interface{} { return &r.cfg.Collect }

func (r *CollectRunner) AddOptions(vroot *viper.Viper) error {

	cfg := r.GetParent().GetNamedConfig(r.name).(*collect.Config)

	if err := root.SetDefaultConfig(vroot, root.GetRunnerName(r), cfg); err != nil {
		panic(err)
	}

	r.vroot = vroot

	f := r.cmd.Flags()

	f.StringVar(
		&cfg.DBSource, "dbsource", cfg.DBSource, `
results will be recorded to this data source. empty (default) disables
collection, :memory: collects (and reports) using in memory store.`)

	f.IntVarP(
		&cfg.NumTransactions, "transactions", "x", cfg.NumTransactions, `
the total number of transactions to issue. note that this is rounded to be an
even multiple of t * a. a minimum of t * a transactions will be issued
regardless`,
	)

	f.Int64VarP(&cfg.StartBlock, "startblock", "s", -1,
		`first block to collect. -1 starts at the current head`)
	f.Int64Var(&cfg.EndBlock, "endblock", -1, `
last block to collect. set -1 (the default) to end only when all expected
transactions are mined`)

	f.DurationVar(
		&cfg.CollectRate, "collect-rate", cfg.CollectRate, `
rate to collect blocks, also effectively the window over which the tps & tpb are
averaged for the progress bar. ignored if dbsource is not set (set to :memory:
if you don't want the results but do want the rate indicators)`)

	return nil
}

func (r *CollectRunner) ProcessConfigxxx() error {
	// Call parent first, results in root down processing order.
	r.GetParent().ProcessConfig()
	r.cfgDir = filepath.Dir(r.vroot.ConfigFileUsed())
	ReconcileOptions(r.cmd, r.vroot.Sub(root.GetRunnerName(r)))
	return nil
}

func (r *CollectRunner) ProcessConfig() error {
	// Call parent first, results in root down processing order.
	r.cfgDir = filepath.Dir(r.vroot.ConfigFileUsed())
	ReconcileOptions(r.cmd, r.vroot.Sub(root.GetRunnerName(r)))
	return nil
}

func (r *CollectRunner) Run(cmd *cobra.Command, args []string) {

	c, err := collect.NewCollector(context.Background(), r.cfgDir, r)
	cobra.CheckErr(err)
	c.Run()
}

func NewCollectCmd(parent Runner, cfg *Config) Runner {
	r := &CollectRunner{
		Runnable{
			name:   collect.ConfigName,
			parent: parent,
			cmd: &cobra.Command{
				Use:   "collect",
				Short: "collect the results from a range of blocks",
				Long:  "long description ...",
			},
			cfg: cfg,
		},
	}

	r.cmd.Run = r.Run
	// r.cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
	// 	return r.ProcessConfig()
	// }

	return r
}
