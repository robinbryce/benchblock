package loader

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
)

const (
	GetSetAddABI = `[ { "constant": false, "inputs": [ { "internalType": "uint256", "name": "x", "type": "uint256" } ], "name": "add", "outputs": [], "payable": false, "stateMutability": "nonpayable", "type": "function" }, { "constant": false, "inputs": [ { "internalType": "uint256", "name": "x", "type": "uint256" } ], "name": "set", "outputs": [], "payable": false, "stateMutability": "nonpayable", "type": "function" }, { "constant": true, "inputs": [], "name": "get", "outputs": [ { "internalType": "uint256", "name": "retVal", "type": "uint256" } ], "payable": false, "stateMutability": "view", "type": "function" } ]`
	GetSetAddBin = "0x608060405234801561001057600080fd5b50610126806100206000396000f3fe6080604052348015600f57600080fd5b50600436106059576000357c0100000000000000000000000000000000000000000000000000000000900480631003e2d214605e57806360fe47b11460895780636d4ce63c1460b4575b600080fd5b608760048036036020811015607257600080fd5b810190808035906020019092919050505060d0565b005b60b260048036036020811015609d57600080fd5b810190808035906020019092919050505060de565b005b60ba60e8565b6040518082815260200191505060405180910390f35b806000540160008190555050565b8060008190555050565b6000805490509056fea265627a7a72315820c8bd9d7613946c0a0455d5dcd9528916cebe6d6599909a4b2527a8252b40d20564736f6c634300050b0032"
)

var (
	big0 = big.NewInt(0)
	big1 = big.NewInt(1)
)

// Config configures load for the runners
type Config struct {
	TPS int `mapstructure:"TPS"`

	// Each thread issues transactions to its own client connection. If
	// SingleNode is set, then those connections are all to the same node.
	// Otherwise each client connection connects to a different node.
	Threads int `mapstructure:"THREADS"`
	// How many accounts to use on each thread. defaults to 5
	ThreadAccounts int `mapstructure:"THREADACCOUNTS"`

	// The target total number of transactions. At least NumThreads * AccountsPerThread tx are issued.
	NumTransactions int `mapstructure:"TRANSACTIONS"`

	GasLimit uint64 `mapstructure:"GASLIMIT"`

	PrivateFor string `mapstructure:"PRIVATEFOR,omitempty"` // "base64pub:base64pub:..." defaults empty - no private tx

	DBSource string `mapstructure:"DBSOURCE,omitempty"`

	// SingleNode is set if all transactions should be issued to the same
	// node. This isolates the load for transaction submission to a single node.
	// We then get an idea of how efficiently the remaining nodes reach
	// consensus on those transactions. Assuming that node is at least able to
	// effectively gossip those transactions. If there is a significant
	// difference with and without this set then the nodes are likely just under
	// resourced.
	SingleNode bool `mapstructure:"SINGLENODE"`

	// If true, confirm every transaction in a batch before doing the next batch.
	CheckReceipts bool `mapstructure:"CHECK_RECEIPTS"`
	Retries       int  `mapstructure:"RETRIES"`

	EthEndpoint     string `mapstructure:"ETH"`
	TesseraEndpoint string `mapstructure:"TESERA"`

	ClientTimeout   time.Duration `mapstructure:"CLIENT_TIMEOUT"`
	ExpectedLatency time.Duration `mapstructure:"EXPECTED_LATENCY"`

	DeployGasLimit uint64        `mapstructure:"DEPLOY_GASLIMIT"`
	DeployKey      string        `mapstructure:"DEPLOY_KEY"` // needs to have funds even for quorum, used to deploy contract
	RunOne         bool          `mapstructure:"RUN_ONE"`
	NoProgress     bool          `mapstructure:"NO_PROGRESS"`
	CollectRate    time.Duration `mapstructure:"COLLECT_RATE"`
}

var defaultCfg = Config{
	TPS:             220,
	Threads:         10,
	ThreadAccounts:  5,
	NumTransactions: 5000,
	GasLimit:        60000,
	PrivateFor:      "",
	DBSource:        "",
	SingleNode:      false,
	CheckReceipts:   false,
	Retries:         10,
	EthEndpoint:     "http://127.0.0.1:8300/",
	TesseraEndpoint: "",
	ClientTimeout:   60 * time.Second,
	ExpectedLatency: 10 * time.Second,
	DeployGasLimit:  600000,
	DeployKey:       "",
	RunOne:          false,
	NoProgress:      true,
	CollectRate:     10 * time.Second,
}

// Adder runs a load of 'add' contract calls, according to the load
// configuration, to the standard get/set/add example solidity contract.
type Adder struct {
	cfg        *Config
	db         *BlockDB
	pbTxIssued *mpb.Bar
	pbTxMined  *mpb.Bar

	limiter *time.Ticker
	// One AccountSet per thread
	accounts []AccountSet
	// One connection per thread
	ethC    []*Client
	ethCUrl []string

	getSetAdd      *bind.BoundContract
	address        common.Address
	collectLimiter *time.Ticker
	numCollected   int
}

// AcountSet groups a set of accounts together. Each thread works with its own
// account set. The wallet keys are generated fresh each run so that we know the
// nonces are ours to manage.
type AccountSet struct {
	Wallets []common.Address
	Keys    []*ecdsa.PrivateKey
	Auth    []*bind.TransactOpts
}

func SetViperDefaults(v *viper.Viper) {
	v.SetDefault("TPS", defaultCfg.TPS)
	v.SetDefault("THREADS", defaultCfg.Threads)
	v.SetDefault("THREADACCOUNTS", defaultCfg.ThreadAccounts)
	v.SetDefault("TRANSACTIONS", defaultCfg.NumTransactions)
	v.SetDefault("GASLIMIT", defaultCfg.GasLimit)

	v.SetDefault("PRIVATEFOR", defaultCfg.PrivateFor)

	v.SetDefault("DBSource", defaultCfg.DBSource)

	v.SetDefault("SINGLENODE", defaultCfg.SingleNode)
	v.SetDefault("CHECK_RECIEPTS", defaultCfg.CheckReceipts)
	v.SetDefault("RETRIES", defaultCfg.Retries)

	v.SetDefault("ETH", defaultCfg.EthEndpoint)
	v.SetDefault("TESSERA", defaultCfg.TesseraEndpoint)

	v.SetDefault("CLIENT_TIMEOUT", defaultCfg.ClientTimeout)
	v.SetDefault("EXPECTED_LATENCY", 3*time.Second)

	v.SetDefault("DEPLOY_GASLIMIT", defaultCfg.DeployGasLimit)
	v.SetDefault("DEPLOY_KEY", defaultCfg.DeployKey)
	v.SetDefault("RUN_ONE", defaultCfg.RunOne)
	v.SetDefault("PROGRESS", defaultCfg.NoProgress)
	v.SetDefault("COLLECT_RATE", defaultCfg.CollectRate)
}

func AddOptions(cmd *cobra.Command, cfg *Config) {

	f := cmd.PersistentFlags()
	f.IntVarP(
		&cfg.TPS, "tps", "r", defaultCfg.TPS,
		"the maximum transactions per second to issue transactions",
	)
	f.IntVarP(
		&cfg.Threads, "threads", "t", defaultCfg.Threads,
		"create this many client conections and run each in its own thread")
	f.IntVarP(
		&cfg.ThreadAccounts, "threadaccounts", "a", defaultCfg.ThreadAccounts, `
each thread will issue transactions in batches of this size. a unique account is
created for each batch entry. #accounts total = t * a`,
	)
	f.IntVarP(
		&cfg.NumTransactions, "transactions", "n", defaultCfg.NumTransactions, `
the total number of transactions to issue. note that this is rounded to be an
even multiple of t * a. a minimum of t * a transactions will be issued
regardless`,
	)
	f.Uint64VarP(
		&cfg.GasLimit, "gaslimit", "g", defaultCfg.GasLimit,
		"the gaslimit to set for each transaction")
	f.StringVar(
		&cfg.PrivateFor, "privatefor", "", `
all transactions will be privatefor the ':' separated list of keys (quorum
only).  if not set, the transactions will be public`)
	f.StringVar(
		&cfg.DBSource, "dbsource", "", `results will be recorded to this data source. empty (default) disables collection, :memory: collects (and reports) using in memory store.`)

	f.BoolVar(
		&cfg.SingleNode, "singlenode", false, "if set all clients will connect to the same node")
	f.BoolVar(
		&cfg.CheckReceipts, "check-reciepts", false, `
if set, threads will verify the transactions issued for each batch at the end of
each batch. otherwise transactions are not verified`)
	f.IntVarP(
		&cfg.Retries, "retries", "c", defaultCfg.Retries,
		`
number of retries for any eth request (a geometric backoff is applied between each try)`)
	f.StringVarP(
		&cfg.EthEndpoint, "eth", "e", defaultCfg.EthEndpoint, `
ethereum json rpc endpoint. each thread derives a client url by adding its index
to the port (unless --singlenode is set)`)
	f.StringVar(
		&cfg.TesseraEndpoint, "tessera", defaultCfg.TesseraEndpoint, `
if privatefor is set, this must be the tessera endpoint to which the private
payload can be submitted. each thread derives a client url by adding its index
to the port (unless --singlenode is set)`)
	f.DurationVar(
		&cfg.ClientTimeout, "client-timeout", defaultCfg.ClientTimeout,
		"every eth request sets this timeout")
	f.DurationVar(
		&cfg.ExpectedLatency, "expected-latency", defaultCfg.ExpectedLatency, `
expected latency for mining transactions (anticipated block rate is a good
choice). this just tunes the receipt retry rate, ignored if
--check-receipts is not set`)

	f.BoolVarP(
		&cfg.RunOne, "one", "o", false,
		"loads the configuration and issues a single transaction. use for testing the config")

	f.Uint64Var(
		&cfg.DeployGasLimit, "deploy-gaslimit", defaultCfg.DeployGasLimit,
		"the gaslimit to set for deploying the contract")
	f.StringVar(
		&cfg.DeployKey, "deploy-key", defaultCfg.DeployKey, `the key to use to deploy the contract. (may need to be funded, if not leave unset)`,
	)

	f.BoolVar(
		&cfg.NoProgress, "no-progress", false,
		"disables progress meter")
	f.DurationVar(
		&cfg.CollectRate, "collect-rate", defaultCfg.CollectRate, `
rate to collect blocks, also effectively the window over which the tps & tpb are
averaged for the progress bar. ignored if dbsource is not set (set to :memory:
if you don't want the results but do want the rate indicators)`)

}

func (a AccountSet) Len() int {
	return len(a.Keys)
}

func (a AccountSet) IncNonce(i int) {
	// this will panic if its out of range. that is intentional
	a.Auth[i].Nonce.Add(a.Auth[i].Nonce, big1)
}

// WithTimeout sets a cancelation context for the auth and returns a CancelFunc
// for it which cleans up the auth. It is NOT SAFE to have multiple contexts
// outstanding for the same auth index.
func (a AccountSet) WithTimeout(parent context.Context, d time.Duration, i int) (context.Context, context.CancelFunc) {

	ctx, cancel := context.WithTimeout(context.Background(), d)
	return ctx, func() {
		cancel()
		a.Auth[i].Context = nil
	}
}

func NewAccountSet(ctx context.Context, ethC *ethclient.Client, cfg *Config, n int) (AccountSet, error) {

	a := AccountSet{}
	a.Wallets = make([]common.Address, n)
	a.Keys = make([]*ecdsa.PrivateKey, n)
	a.Auth = make([]*bind.TransactOpts, n)

	var nonce uint64
	var err error
	for i := 0; i < n; i++ {

		// Generate an epheral account key for the test. This guarantees the nonces are ours to manage.
		if a.Keys[i], err = crypto.GenerateKey(); err != nil {
			return AccountSet{}, err
		}
		pub := a.Keys[i].PublicKey

		// derive the wallet address from the private key
		pubBytes := elliptic.Marshal(secp256k1.S256(), pub.X, pub.Y)
		pubHash := crypto.Keccak256(pubBytes[1:]) // skip the compression indicator
		copy(a.Wallets[i][:], pubHash[12:])       // wallet address is the trailing 20 bytes

		a.Auth[i] = bind.NewKeyedTransactor(a.Keys[i])
		if cfg.PrivateFor != "" {
			a.Auth[i].PrivateFor = strings.Split(cfg.PrivateFor, ":")
		}
		a.Auth[i].GasLimit = cfg.GasLimit
		a.Auth[i].GasPrice = big.NewInt(0)

		if nonce, err = ethC.PendingNonceAt(ctx, a.Wallets[i]); err != nil {
			return AccountSet{}, err
		}
		a.Auth[i].Nonce = big.NewInt(int64(nonce))
	}
	return a, nil
}

func NewAdder(ctx context.Context, cfg *Config) (Adder, error) {

	var err error

	lo := Adder{cfg: cfg}
	if delta := lo.cfg.TruncateTargetTransactions(); delta != 0 {
		fmt.Printf(
			"adjusted target number of transactions from %d to %d\n",
			lo.cfg.NumTransactions+delta, lo.cfg.NumTransactions)
	}

	// Are we collecting results for analysis?
	if lo.cfg.DBSource != "" {
		if lo.db, err = NewBlockDB(lo.cfg.DBSource, true /*share*/); err != nil {
			return Adder{}, err
		}
	}

	lo.limiter = time.NewTicker(time.Second / time.Duration(lo.cfg.TPS))

	lo.collectLimiter = time.NewTicker(lo.cfg.CollectRate)

	qu, err := url.Parse(lo.cfg.EthEndpoint)
	if err != nil {
		return Adder{}, err
	}
	quHostname := qu.Hostname()
	baseQuorumPort, err := strconv.Atoi(qu.Port())
	if err != nil {
		return Adder{}, err
	}

	var tu *url.URL
	var tuHostname string
	var baseTesseraPort int
	if lo.cfg.TesseraEndpoint != "" {
		tu, err = url.Parse(lo.cfg.TesseraEndpoint)
		if err != nil {
			return Adder{}, err
		}

		tuHostname = tu.Hostname()
		baseTesseraPort, err = strconv.Atoi(tu.Port())
		if err != nil {
			return Adder{}, err
		}
	}

	lo.accounts = make([]AccountSet, lo.cfg.Threads)
	lo.ethC = make([]*Client, lo.cfg.Threads)
	lo.ethCUrl = make([]string, lo.cfg.Threads)

	var tx *types.Transaction

	var deployKey *ecdsa.PrivateKey
	parsed, err := abi.JSON(strings.NewReader(GetSetAddABI))
	if err != nil {
		return Adder{}, err
	}

	if lo.cfg.DeployKey != "" {
		deployKey, err = crypto.HexToECDSA(lo.cfg.DeployKey)
		if err != nil {
			return Adder{}, err
		}
	}
	if deployKey == nil {
		// This will likely fail as normal quorum requires balance to deploy
		// event tho gasprice is 0
		deployKey, err = crypto.GenerateKey()
		if err != nil {
			return Adder{}, err
		}
	}

	// if we have multiple exposed nodes we can do this
	// ethEndpoint := fmt.Sprintf("http://localhost:220%02d", i)
	// tessEndpoint := fmt.Sprintf("http://localhost:90%02d", 8+i)
	// Otherwise we hit the same node with multiple clients

	for i := 0; i < lo.cfg.Threads; i++ {
		qu.Host = fmt.Sprintf("%s:%d", quHostname, baseQuorumPort+(i%lo.cfg.Threads))
		var tuEndpoint string
		if tu != nil {
			tu.Host = fmt.Sprintf("%s:%d", tuHostname, baseTesseraPort+(i%lo.cfg.Threads))
			tuEndpoint = tu.String()
		}

		lo.ethC[i], err = NewClient(qu.String(), tuEndpoint, lo.cfg.ClientTimeout)
		if err != nil {
			return Adder{}, err
		}
		lo.ethCUrl[i] = qu.String()

		fmt.Printf("building account set for client[%d]: %s\n", i, qu.String())

		lo.accounts[i], err = NewAccountSet(ctx, lo.ethC[i].Client, lo.cfg, lo.cfg.ThreadAccounts)
		if err != nil {
			return Adder{}, err
		}
	}

	deployAuth := bind.NewKeyedTransactor(deployKey)

	deployAuth.GasLimit = uint64(lo.cfg.DeployGasLimit)
	deployAuth.GasPrice = big0
	lo.address, tx, lo.getSetAdd, err = bind.DeployContract(
		deployAuth, parsed, common.FromHex(GetSetAddBin), lo.ethC[0])
	if err != nil {
		return Adder{}, err
	}
	if ok := CheckReceipt(lo.ethC[0].Client, tx, lo.cfg.Retries, lo.cfg.ExpectedLatency); !ok {
		return Adder{}, fmt.Errorf("failed to deploy contract")
	}

	// num-tx / num-threads

	return lo, nil
}

func (a *Adder) Run() {

	var wg sync.WaitGroup
	// p := mpb.New(mpb.WithWaitGroup(&wg))
	p := mpb.New()

	// Are we running the progress meter ?
	if !a.cfg.NoProgress {
		// wg.Add(2)

		// pb.StartNew(lo.cfg.NumTransactions)
		a.pbTxIssued = p.AddBar(
			int64(a.cfg.NumTransactions), mpb.PrependDecorators(
				decor.Name("sent", decor.WCSyncSpace),
				decor.CurrentNoUnit("%d", decor.WCSyncSpace),
				decor.AverageSpeed(0, "%f.2/s", decor.WCSyncSpace),
				decor.Elapsed(decor.ET_STYLE_MMSS, decor.WCSyncSpace),
			),
		)
		a.pbTxMined = p.AddBar(
			int64(a.cfg.NumTransactions),
			mpb.PrependDecorators(
				decor.Name("mined", decor.WCSyncSpace),
				decor.CurrentNoUnit("%d", decor.WCSyncSpace),
				decor.AverageSpeed(0, "%f.2/s", decor.WCSyncSpace),
				decor.Elapsed(decor.ET_STYLE_MMSS, decor.WCSyncSpace),
			),
		)
	}

	for i := 0; i < a.cfg.Threads; i++ {
		wg.Add(1)
		go a.adder(a.ethC[i].Client, &wg, fmt.Sprintf("client-%d", i), i)
	}

	if a.db != nil {
		wg.Add(1)
		go a.collector(a.ethC[0], &wg, fmt.Sprintf("client-%d", 0), 0)
	}
	wg.Wait()
	fmt.Printf("sent: %d, mined: %d\n", a.pbTxIssued.Current(), a.pbTxIssued.Current())
	// p.Wait()
}

// RunOne is provided for dignostic purposes. It issues a single transaction
// using the first account in the first account set for the provided
// configuration.
func (lo *Adder) RunOne() error {

	auth := lo.accounts[0].Auth[0]
	wallet := lo.accounts[0].Wallets[0]
	ethC := lo.ethC[0]

	var nonce uint64
	var err error
	nonce, err = ethC.PendingNonceAt(context.Background(), wallet)
	if err != nil {
		return err
	}
	auth.Nonce = big.NewInt(int64(nonce))

	var tx *types.Transaction
	tx, err = lo.getSetAdd.Transact(auth, "add", big.NewInt(3))
	if err != nil {
		return err
	}
	if ok := CheckReceipt(ethC.Client, tx, lo.cfg.Retries, lo.cfg.ClientTimeout); !ok {
		return fmt.Errorf("transaction %s failed or not completed in %v", tx.Hash().Hex(), lo.cfg.ClientTimeout)
	}
	return nil
}

func (a *Adder) collector(ethC *Client, wg *sync.WaitGroup, banner string, ias int) {

	defer wg.Done()

	var err error
	var raw json.RawMessage
	var s string
	var lastBlock, blockNumber int64
	var block *types.Block
	numCollected := 0

	getBlockNumber := func() (int64, error) {

		var num int64
		// initialise last block number
		ctx, cancel := context.WithTimeout(context.Background(), a.cfg.ClientTimeout)
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

	if lastBlock, err = getBlockNumber(); err != nil {
		return
	}

	for range a.collectLimiter.C {

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
			if a.pbTxIssued == nil {
				fmt.Printf("no more blocks since %d\n", blockNumber)
			}
			continue
		}

		for i := lastBlock + 1; i <= blockNumber; i++ {

			ctx, cancel := context.WithTimeout(context.Background(), a.cfg.ClientTimeout)
			block, err = GetBlockByNumber(ctx, ethC.Client, a.cfg.Retries, i)
			cancel()
			if err != nil {
				fmt.Printf("error getting block %d: %v\n", i, err)
				return
			}

			h := block.Header()
			if err = a.db.Insert(block, h); err != nil {
				println(fmt.Errorf("inserting block %v: %w", h.Number, err).Error())
			}
			lastBlock = i

			// could actually capture and reconcile them if we wanted, for now just count them
			ntx := len(block.Transactions())
			a.pbTxMined.IncrBy(ntx)
			numCollected += ntx
			if numCollected >= a.cfg.NumTransactions {
				fmt.Printf("collection complete: %d\n", numCollected)
				a.pbTxMined.SetTotal(int64(a.cfg.NumTransactions), true)
				return
			}
		}
	}
}

func (lo *Adder) adder(ethC *ethclient.Client, wg *sync.WaitGroup, banner string, ias int) {

	defer wg.Done()

	var tx *types.Transaction

	// Note: NumTransactions is adjusted by TruncateTargetTransactions so
	// everything works out as whole numbers. And so that
	//  NumTransactions >= lo.cfg.NumThreads * lo.cfg.AccountsPerThread
	numBatches := lo.cfg.NumTransactions / (lo.cfg.Threads * lo.cfg.ThreadAccounts)

	// Each batch issues on tx per account. The batching is only worth it if
	// CheckBatchReciepts is true (it cleans up the picture by reducing the eth
	// rpc load  on the node)
	batch := make([]*types.Transaction, lo.cfg.ThreadAccounts)

	var err error

	// First, initialise the nonces for the transactors in the AccountSet, and also set the ctx in the auth's
	for i := 0; i < lo.cfg.ThreadAccounts; i++ {
		var nonce uint64
		ctx, cancel := context.WithTimeout(context.Background(), lo.cfg.ClientTimeout)
		nonce, err = ethC.PendingNonceAt(ctx, lo.accounts[ias].Wallets[i])
		cancel()
		if err != nil {
			fmt.Printf("terminating client for %s. error initialsing nonce: %v\n", lo.ethCUrl[ias], err)
		}

		lo.accounts[ias].Auth[i].Nonce = big.NewInt(int64(nonce))
	}

	for r := 0; r < numBatches; r++ {

		if lo.pbTxIssued == nil && lo.pbTxMined == nil {
			fmt.Printf("%s: batch %d, node %s\n", banner, r, lo.ethCUrl[ias])
		}

		for i := 0; i < lo.cfg.ThreadAccounts; i++ {
			if lo.limiter != nil {
				<-lo.limiter.C
			}

			// Set the ctx for the auth
			_, cancel := lo.accounts[ias].WithTimeout(context.Background(), lo.cfg.ClientTimeout, i)
			tx, err = lo.getSetAdd.Transact(lo.accounts[ias].Auth[i], "add", big.NewInt(2))
			cancel()
			if err != nil {
				fmt.Printf("terminating client for %s. error from transact: %v\n", lo.ethCUrl[ias], err)
				return
			}
			if lo.pbTxIssued != nil {
				lo.pbTxIssued.Increment()
			}

			lo.accounts[ias].IncNonce(i)
			batch[i] = tx
		}
		if lo.cfg.CheckReceipts {
			for i := 0; i < lo.cfg.ThreadAccounts; i++ {
				if ok := CheckReceipt(ethC, batch[i], lo.cfg.Retries, lo.cfg.ExpectedLatency); !ok {
					fmt.Printf("terminating client for %s. no valid receipt found for tx: %s\n",
						lo.ethCUrl[ias], tx.Hash().Hex())
				}
			}
		}
	}
}

// TruncageTargetTransactions conditions the requested NumTransactions so that
// all threads serve the same number of transactions. Each thread will serve at
// least one tx.
func (cfg *Config) TruncateTargetTransactions() int {
	// 1 tx per account per thread is our minimum
	x := cfg.Threads * cfg.ThreadAccounts
	y := cfg.NumTransactions % x

	if y == 0 {
		return 0
	}

	var n int
	if x == y {
		// NumTransactions to small to make sense
		n, y = x, 0
	} else {
		n = cfg.NumTransactions - y
	}
	delta := cfg.NumTransactions - n

	cfg.NumTransactions = n

	return delta
}
