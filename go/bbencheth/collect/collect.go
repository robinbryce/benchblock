package collect

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/robinbryce/blockbench/bbencheth/client"
	"github.com/robinbryce/blockbench/bbencheth/root"
)

const (
	ConfigName = "collect"
)

type Collector struct {
	ConfigDir  string
	rootCfg    *root.Config
	collectCfg *Config
	db         *BlockDB
	pb         *client.TransactionProgress

	c              *client.Client
	collectLimiter *time.Ticker
}

type Config struct {
	DBSource        string
	StartBlock      int64
	EndBlock        int64
	NumTransactions int
	CollectRate     time.Duration `mapstructure:"collect_rate"`
}

func NewConfigCollect() Config {
	cfg := Config{}
	cfg.SetDefaults()
	return cfg
}

func (cfg *Config) SetDefaults() {
	cfg.DBSource = ":memory:"
	cfg.StartBlock = -1
	cfg.EndBlock = -1
	cfg.NumTransactions = 0
	cfg.CollectRate = 10 * time.Second
}

type CollectorOption func(*Collector)

// WithProgress enables a progress bar for the collector. Pass a nil progress
// to use the internal default. Pass a previously created instance to share a
// progress meter. To disable progress entirely, just don't supply this option.
func WithProgress(pb *client.TransactionProgress) CollectorOption {
	return func(c *Collector) {
		// If the caller provided a pre configured progress meter, use it as is.
		// Otherwise create one just for tracking mined transactions.
		if pb == nil {
			pb = client.NewTransactionProgress(c.collectCfg.NumTransactions, client.WithMinedProgress())
		}
		c.pb = pb
	}
}

func NewCollector(ctx context.Context, cfgDir string, r root.Runner, opts ...CollectorOption) (*Collector, error) {
	var err error

	c := &Collector{
		rootCfg:    r.GetNamedConfig(root.ConfigName).(*root.Config),
		collectCfg: r.GetNamedConfig(ConfigName).(*Config),
		ConfigDir:  cfgDir,
	}

	for _, opt := range opts {
		opt(c)
	}

	// Progress is disabled entirely if the WithProgress option is not
	// supplied. This is most cleanly accomplished with NoOp progress
	if c.pb == nil {
		c.pb = client.NewTransactionProgress(c.collectCfg.NumTransactions)
	}

	if c.collectCfg.DBSource == "" {
		return c, fmt.Errorf(
			"dbsource is required by collector (try :memory: if you just want to wait for completion")
	}
	if c.db, err = NewBlockDB(c.collectCfg.DBSource, true /*share*/); err != nil {
		fmt.Printf("failed creating db (%s): %v\n", c.collectCfg.DBSource, err)
		return nil, err
	}

	if c.rootCfg.EthEndpoint == "" {
		return nil, fmt.Errorf("ethendpoint is a required option")
	}

	c.collectLimiter = time.NewTicker(c.collectCfg.CollectRate)

	qu, err := url.Parse(c.rootCfg.EthEndpoint)
	if err != nil {
		return nil, err
	}

	quHostname, err := c.resolveHost(qu.Hostname())
	if err != nil {
		return nil, err
	}

	baseQuorumPort := c.rootCfg.BasePort
	if baseQuorumPort == 0 {
		baseQuorumPort, err = strconv.Atoi(qu.Port())
		if err != nil {
			return nil, err
		}
	}
	qu.Host = fmt.Sprintf("%s:%d", quHostname, baseQuorumPort)
	c.c, err = client.NewClient(qu.String(), "", c.rootCfg.ClientTimeout)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Collector) Run() {

	c.Collect(c.c, nil, fmt.Sprintf("client-%d", 0), 0)
	if mined := c.pb.CurrentMined(); mined != -1 {
		fmt.Printf("mined: %d\n", mined)
	}
}

func (c *Collector) Collect(ethC *client.Client, wg *sync.WaitGroup, banner string, ias int) {

	if wg != nil {
		defer wg.Done()
	}

	var err error
	var raw json.RawMessage
	var s string
	var lastBlock, blockNumber int64
	var block *types.Block

	getBlockNumber := func() (int64, error) {

		var num int64
		ctx, cancel := context.WithTimeout(context.Background(), c.rootCfg.ClientTimeout)
		err = ethC.RPC.CallContext(ctx, &raw, "eth_blockNumber")
		cancel()
		if err != nil {
			fmt.Printf("error calling eth_blockNumber rpc: %v\n", err)
			return 0, err
		}
		if err = json.Unmarshal(raw, &s); err != nil {
			fmt.Printf("error decoding eth_blockNumber response: %v\n", err)
			return 0, err
		}
		num, err = strconv.ParseInt(s, 0, 64)
		if err != nil {
			fmt.Printf("error decoding Result field on eth_blockNumber response: %v\n", err)
			return 0, err
		}
		return num, nil
	}

	// initialise last block number
	lastBlock = c.collectCfg.StartBlock
	if lastBlock == -1 {
		if lastBlock, err = getBlockNumber(); err != nil {
			return
		}
	}

	for range c.collectLimiter.C {

		if blockNumber, err = getBlockNumber(); err != nil {
			return
		}

		// re-orgs might mean we go backwards on some consensus algs, we
		// don't really try to deal with that here. using <= is effectively
		// ignoring the issue.
		if blockNumber <= lastBlock {
			if blockNumber < lastBlock {
				fmt.Printf("re-org ? new head %d is < %dn", blockNumber, lastBlock)
			}
			fmt.Printf("no more blocks since %d\n", blockNumber)
			continue
		}

		for i := lastBlock + 1; i <= blockNumber; i++ {

			ctx, cancel := context.WithTimeout(context.Background(), c.rootCfg.ClientTimeout)
			block, err = client.GetBlockByNumber(ctx, ethC.Client, c.rootCfg.Retries, i)
			cancel()
			if err != nil {
				fmt.Printf("error getting block %d: %v\n", i, err)
				return
			}

			h := block.Header()
			if err = c.db.Insert(block, h); err != nil {
				println(fmt.Errorf("inserting block %v: %w", h.Number, err).Error())
			}
			lastBlock = i

			// could actually capture and reconcile them against the accounts we created if we wanted, for now just count them.
			ntx := len(block.Transactions())
			if c.pb.MinedComplete(ntx) || (c.collectCfg.EndBlock != -1 && lastBlock > c.collectCfg.EndBlock) {
				fmt.Printf("collection complete. block %d, mined: %d\n", lastBlock, c.pb.NumMined())
				return
			}
		}
	}
}

func (c *Collector) resolveHost(host string) (string, error) {
	if !c.rootCfg.ResolveHosts {
		return host, nil
	}
	addrs, err := net.LookupIP(host)
	if err != nil {
		return "", err
	}
	if len(addrs) != 1 {
		return "", fmt.Errorf("cant resolve ambigous host. could be any of: %v", addrs)
	}
	return addrs[0].String(), nil
}
