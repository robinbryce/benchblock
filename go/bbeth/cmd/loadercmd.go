package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/robinbryce/benchblock/bbencheth/client"
	"github.com/robinbryce/benchblock/bbencheth/collect"
	"github.com/robinbryce/benchblock/bbencheth/load"
	"github.com/robinbryce/benchblock/bbencheth/root"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type LoaderRunner struct {
	Runnable
	collectStartBlock int64
}

func (r *LoaderRunner) GetConfig() interface{} { return &r.cfg.Load }

func (r *LoaderRunner) AddOptions(vroot *viper.Viper) error {

	cfg := r.GetNamedConfig(load.ConfigName).(*load.Config)

	if err := root.SetDefaultConfig(vroot, root.GetRunnerName(r), cfg); err != nil {
		panic(err)
	}

	r.vroot = vroot

	f := r.cmd.PersistentFlags()
	// f.SetNormalizeFunc(NormalizeOptions)

	f.IntVarP(
		&cfg.TPS, "tps", "r", cfg.TPS,
		"the maximum transactions per second to issue transactions",
	)
	f.IntVarP(
		&cfg.Threads, "threads", "t", cfg.Threads,
		"create this many client conections and run each in its own thread")
	f.IntVarP(
		&cfg.Nodes, "nodes", "n", cfg.Nodes,
		"by default threads is assumed to be 1 or the node count. To get n clients per node, set nodes to the node count and threads to nodes * n")

	f.IntVarP(
		&cfg.ThreadAccounts, "threadaccounts", "a", cfg.ThreadAccounts, `
		each thread will issue transactions in batches of this size. a unique account is
		created for each batch entry. #accounts total = t * a`,
	)
	f.IntVarP(
		&cfg.NumTransactions, "transactions", "x", cfg.NumTransactions, `
		the total number of transactions to issue. note that this is rounded to be an
		even multiple of t * a. a minimum of t * a transactions will be issued
		regardless`,
	)
	f.Uint64VarP(
		&cfg.GasLimit, "gaslimit", "g", cfg.GasLimit,
		"the gaslimit to set for each transaction")
	f.StringVar(
		&cfg.PrivateFor, "privatefor", "", `
		all transactions will be privatefor the ':' separated list of keys (quorum
		only).  if not set, the transactions will be public`)

	f.BoolVar(
		&cfg.SingleNode, "singlenode", false, "if set all clients will connect to the same node (regardless of other options)")

	f.BoolVar(
		&cfg.CheckReceipts, "check-reciepts", false, `
	if set, threads will verify the transactions issued for each batch at the
	end of each batch. otherwise transactions are not verified`)
	f.StringVar(
		&cfg.TesseraEndpoint, "tessera", cfg.TesseraEndpoint, `
	if privatefor is set, this must be the tessera endpoint to which the private
	payload can be submitted. each thread derives a client url by adding its
	index to the port (unless --singlenode is set)`)

	f.StringVar(
		&cfg.StaticNodes, "staticnodes", cfg.StaticNodes, `
	An alternative to --eth. If provided, node hosts are read from the file.
	Its assumed to be the same format as static-nodes.json. Only the hostname
	(or IP) field of the url is significant. The port is set seperately
	(baseport) and must be the same for all nodes.`)
	f.IntVar(
		&cfg.BaseTesseraPort, "basetesseraport", cfg.BaseTesseraPort,
		`The first port if --eth is used. If using --staticnodes all nodes be on this port`)
	f.DurationVar(
		&cfg.ExpectedLatency, "expected-latency", cfg.ExpectedLatency, `
	expected latency for mining transactions (anticipated block rate is a good
	choice). this just tunes the receipt retry rate, ignored if
	--check-receipts is not set`)

	f.BoolVarP(
		&cfg.RunOne, "one", "o", false,
		"loads the configuration and issues a single transaction. use for testing the config")

	f.Uint64Var(
		&cfg.DeployGasLimit, "deploy-gaslimit", cfg.DeployGasLimit,
		"the gaslimit to set for deploying the contract")
	f.StringVar(
		&cfg.DeployKey, "deploy-key", cfg.DeployKey, `the key to use to deploy the contract. (may need to be funded, if not leave unset)`,
	)

	f.Int64VarP(&r.collectStartBlock, "startblock", "s", -1,
		`first block to collect. -1 starts at the current head`)

	return nil
}

func (r *LoaderRunner) ProcessConfig() error {

	r.cfgDir = filepath.Dir(r.vroot.ConfigFileUsed())
	ReconcileOptions(r.cmd, r.vroot.Sub(root.GetRunnerName(r)))
	return nil
}

func (r *LoaderRunner) Run(cmd *cobra.Command, args []string) {

	cfg := r.GetNamedConfig(load.ConfigName).(*load.Config)
	rootCfg := r.GetNamedConfig(root.ConfigName).(*root.Config)

	var collectorOpts []collect.CollectorOption
	var opts []load.LoaderOption

	if delta := cfg.TruncateTargetTransactions(); delta != 0 {
		fmt.Printf(
			"adjusted target number of transactions from %d to %d\n",
			cfg.NumTransactions+delta, cfg.NumTransactions)
	}

	collectCfg := r.GetParent().GetNamedConfig(collect.ConfigName).(*collect.Config)

	if !rootCfg.NoProgress {

		progressOpts := []client.ProgressOption{client.WithIssuedProgress()}
		if collectCfg.DBSource != "" {
			progressOpts = append(progressOpts, client.WithMinedProgress())
		}
		pb := client.NewTransactionProgress(cfg.NumTransactions, progressOpts...)

		// doesn't get used if DBSource == ""
		collectorOpts = []collect.CollectorOption{collect.WithProgress(pb)}

		opts = []load.LoaderOption{load.WithProgress(pb)}
	}

	// By default the collector gets the start block from the chain. It is
	// theoretically racy to do so but I've never seen the first tx mine fast
	// enough to be missed.
	var collector *collect.Collector
	if collectCfg.DBSource != "" {
		var err error

		collectCfg.StartBlock = r.collectStartBlock

		collector, err = collect.NewCollector(context.Background(), r.cfgDir, r, collectorOpts...)
		cobra.CheckErr(err)

		opts = append(opts, load.WithCollector(collector))
	}

	a, err := load.NewLoader(context.Background(), r.cfgDir, r, opts...)
	cobra.CheckErr(err)
	if cfg.RunOne {
		err = a.RunOne()
		cobra.CheckErr(err)
		return
	}
	a.Run()
}

func NewLoaderCmd(parent Runner, cfg *Config) Runner {
	r := &LoaderRunner{
		Runnable: Runnable{
			name:   load.ConfigName,
			parent: parent,
			cmd: &cobra.Command{
				Use:   load.ConfigName,
				Short: "A load generator for xxx ethereum networks",
				Long: `
Uses the native go-ethereum libaries to deploy the idiomatic get/set/add
contract and invoke state changing functions to generate transaction load for
ethereum networks`,
			},
			cfg: cfg,
		},
		collectStartBlock: -1,
	}
	r.cmd.Run = r.Run
	// r.cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
	// 	return r.ProcessConfig()
	// }

	return r
}
