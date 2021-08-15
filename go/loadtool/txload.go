package loadtool

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
	"math/big"
	"net/url"
	"os"
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
)

const (
	GetSetAddABI = `[ { "constant": false, "inputs": [ { "internalType": "uint256", "name": "x", "type": "uint256" } ], "name": "add", "outputs": [], "payable": false, "stateMutability": "nonpayable", "type": "function" }, { "constant": false, "inputs": [ { "internalType": "uint256", "name": "x", "type": "uint256" } ], "name": "set", "outputs": [], "payable": false, "stateMutability": "nonpayable", "type": "function" }, { "constant": true, "inputs": [], "name": "get", "outputs": [ { "internalType": "uint256", "name": "retVal", "type": "uint256" } ], "payable": false, "stateMutability": "view", "type": "function" } ]`
	GetSetAddBin = "0x608060405234801561001057600080fd5b50610126806100206000396000f3fe6080604052348015600f57600080fd5b50600436106059576000357c0100000000000000000000000000000000000000000000000000000000900480631003e2d214605e57806360fe47b11460895780636d4ce63c1460b4575b600080fd5b608760048036036020811015607257600080fd5b810190808035906020019092919050505060d0565b005b60b260048036036020811015609d57600080fd5b810190808035906020019092919050505060de565b005b60ba60e8565b6040518082815260200191505060405180910390f35b806000540160008190555050565b8060008190555050565b6000805490509056fea265627a7a72315820c8bd9d7613946c0a0455d5dcd9528916cebe6d6599909a4b2527a8252b40d20564736f6c634300050b0032"
)

var (
	big0 = big.NewInt(0)
	big1 = big.NewInt(1)
)

// LoadConfig configures load for the runners
type LoadConfig struct {
	TargetTPS int

	// Each thread issues transactions to a single client connection. If
	// TargetSingleNode is set, then those connections are all to the same node.
	// Otherwise each client connection connects to a different node.
	NumThreads int
	// How many accounts to use on each thread. defaults to 5
	AccountsPerThread int

	// The target total number of transactions. At least NumThreads * AccountsPerThread tx are issued.
	NumTransactions int

	DefaultGasLimit uint64

	PrivateFor []string // "base64pub:base64pub:..." defaults empty - no private tx

	// TargetSingleNode is set if all transactions should be issued to the same
	// node. This isolates the load for transaction submission to a single node.
	// We then get an idea of how efficiently the remaining nodes reach
	// consensus on those transactions. Assuming that node is at least able to
	// effectively gossip those transactions. If there is a significant
	// difference with and without this set then the nodes are likely just under
	// resourced.
	TargetSingleNode bool

	// If true, confirm every transaction in a batch before doing the next batch.
	CheckBatchReceipts bool
	ReceiptRetries     int

	NodeEndpoint    string
	TesseraEndpoint string

	ClientTimeout   time.Duration
	ExpectedLatency time.Duration

	DeployGasLimit uint64
	DeployKey      string // needs to have funds even for quorum, used to deploy contract
}

// Adder runs a load of 'add' contract calls, according to the load
// configuration, to the standard get/set/add example solidity contract.
type Adder struct {
	cfg     *LoadConfig
	limiter *time.Ticker
	// One AccountSet per thread
	accounts []AccountSet
	// One connection per thread
	ethC      []*ethclient.Client
	ethCUrl   []string
	getSetAdd *bind.BoundContract
	address   common.Address
}

// AcountSet groups a set of accounts together. Each thread works with its own
// account set. The wallet keys are generated fresh each run so that we know the
// nonces are ours to manage.
type AccountSet struct {
	Wallets []common.Address
	Keys    []*ecdsa.PrivateKey
	Auth    []*bind.TransactOpts
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

func NewAccountSet(ctx context.Context, ethC *ethclient.Client, cfg *LoadConfig, n int) (AccountSet, error) {

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
		if cfg.PrivateFor != nil { // empty but not nil is stil private
			a.Auth[i].PrivateFor = make([]string, len(cfg.PrivateFor))
			copy(a.Auth[i].PrivateFor, cfg.PrivateFor)
		}
		a.Auth[i].GasLimit = cfg.DefaultGasLimit
		a.Auth[i].GasPrice = big.NewInt(0)

		if nonce, err = ethC.PendingNonceAt(ctx, a.Wallets[i]); err != nil {
			return AccountSet{}, err
		}
		a.Auth[i].Nonce = big.NewInt(int64(nonce))
	}
	return a, nil
}

func NewAdder(ctx context.Context, cfg *LoadConfig) (Adder, error) {

	lo := Adder{cfg: cfg}
	if delta := lo.cfg.TruncateTargetTransactions(); delta != 0 {
		fmt.Printf(
			"adjusted target number of transactions from %d to %d\n",
			lo.cfg.NumTransactions+delta, lo.cfg.NumTransactions)
	}

	lo.limiter = time.NewTicker(time.Second / time.Duration(lo.cfg.TargetTPS))

	qu, err := url.Parse(lo.cfg.NodeEndpoint)
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

	lo.accounts = make([]AccountSet, lo.cfg.NumThreads)
	lo.ethC = make([]*ethclient.Client, lo.cfg.NumThreads)
	lo.ethCUrl = make([]string, lo.cfg.NumThreads)

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

	for i := 0; i < lo.cfg.NumThreads; i++ {
		qu.Host = fmt.Sprintf("%s:%d", quHostname, baseQuorumPort+(i%lo.cfg.NumThreads))
		var tuEndpoint string
		if tu != nil {
			tu.Host = fmt.Sprintf("%s:%d", tuHostname, baseTesseraPort+(i%lo.cfg.NumThreads))
			tuEndpoint = tu.String()
		}

		lo.ethC[i], err = NewTransactor(qu.String(), tuEndpoint, lo.cfg.ClientTimeout)
		if err != nil {
			return Adder{}, err
		}
		lo.ethCUrl[i] = qu.String()

		lo.accounts[i], err = NewAccountSet(ctx, lo.ethC[i], lo.cfg, lo.cfg.AccountsPerThread)
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
	if ok := CheckReceipt(lo.ethC[0], tx, lo.cfg.ReceiptRetries, lo.cfg.ExpectedLatency); !ok {
		return Adder{}, fmt.Errorf("failed to deploy contract")
	}

	// num-tx / num-threads

	return lo, nil
}

func (a *Adder) Run() {

	var wg sync.WaitGroup
	for i := 0; i < a.cfg.NumThreads; i++ {

		wg.Add(1)
		go a.adder(a.ethC[i], &wg, fmt.Sprintf("client-%d", i), i)
	}
	wg.Wait()
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
	if ok := CheckReceipt(ethC, tx, lo.cfg.ReceiptRetries, lo.cfg.ClientTimeout); !ok {
		return fmt.Errorf("transaction %s failed or not completed in %v", tx.Hash().Hex(), lo.cfg.ClientTimeout)
	}
	return nil
}

func (lo *Adder) adder(ethC *ethclient.Client, wg *sync.WaitGroup, banner string, ias int) {

	defer wg.Done()

	var tx *types.Transaction

	// Note: NumTransactions is adjusted by TruncateTargetTransactions so
	// everything works out as whole numbers. And so that
	//  NumTransactions >= lo.cfg.NumThreads * lo.cfg.AccountsPerThread
	numBatches := lo.cfg.NumTransactions / (lo.cfg.NumThreads * lo.cfg.AccountsPerThread)

	// Each batch issues on tx per account. The batching is only worth it if
	// CheckBatchReciepts is true (it cleans up the picture by reducing the eth
	// rpc load  on the node)
	batch := make([]*types.Transaction, lo.cfg.AccountsPerThread)

	var err error

	// First, initialise the nonces for the transactors in the AccountSet, and also set the ctx in the auth's
	for i := 0; i < lo.cfg.AccountsPerThread; i++ {
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
		fmt.Printf("%s: batch %d, node %s\n", banner, r, lo.ethCUrl[ias])

		for i := 0; i < lo.cfg.AccountsPerThread; i++ {
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

			lo.accounts[ias].IncNonce(i)
			batch[i] = tx
		}
		if lo.cfg.CheckBatchReceipts {
			for i := 0; i < lo.cfg.AccountsPerThread; i++ {
				if ok := CheckReceipt(ethC, batch[i], lo.cfg.ReceiptRetries, lo.cfg.ExpectedLatency); !ok {
					fmt.Printf("terminating client for %s. no valid receipt found for tx: %s\n",
						lo.ethCUrl[ias], tx.Hash().Hex())
				}
			}
		}
	}
}

// NewConfigFromEnv populates a config from the environment. For each
// PublicMember in the Config we read  from the corresponding upper cased
// environment variable. Eg: PUBLIC_MEMBER. This facility allows the load test
// to be run as a unittest.
func NewConfigFromEnv() LoadConfig {
	cfg := LoadConfig{}

	cfg.TargetTPS = intFromEnv("TARGET_TPS", 220)
	cfg.NumThreads = intFromEnv("NUM_THREADS", 10)
	cfg.AccountsPerThread = intFromEnv("NUM_ACCOUNTS_PER_THREAD", 5)
	cfg.NumTransactions = intFromEnv("NUM_TRANSACTIONS", 5000)
	cfg.DefaultGasLimit = uint64(intFromEnv("DEFAULT_GASLIMIT", 60000))

	privateFor := fromEnv("PRIVATE_FOR", "")
	if privateFor != "" {
		cfg.PrivateFor = strings.Split(privateFor, ":")
	}

	n := intFromEnv("TARGET_SINGLE_NODE", 0)
	if n > 0 {
		cfg.TargetSingleNode = true
	}
	cfg.CheckBatchReceipts = intFromEnv("CHECK_BATCH_RECIEPTS", 0) > 0
	cfg.ReceiptRetries = intFromEnv("RECEIPT_RETRIES", 15)

	cfg.NodeEndpoint = fromEnv("NODE_ENDPOINT", "http://127.0.0.1:8300")
	cfg.TesseraEndpoint = fromEnv("NODE_ENDPOINT", "" /* "http://127.0.0.1:9008"*/)

	cfg.ClientTimeout = durationFromEnv("CLIENT_TIMEOUT", 60*time.Second)
	cfg.ExpectedLatency = durationFromEnv("EXPECTED_LATENCY", 3*time.Second)

	cfg.DeployGasLimit = uint64(intFromEnv("DEPLOY_GASLIMIT", 600000))
	cfg.DeployKey = fromEnv("DEPLOY_KEY", "")

	return cfg
}

// TruncageTargetTransactions conditions the requested NumTransactions so that
// all threads serve the same number of transactions. Each thread will serve at
// least one tx.
func (cfg *LoadConfig) TruncateTargetTransactions() int {
	// 1 tx per account per thread is our minimum
	x := cfg.NumThreads * cfg.AccountsPerThread
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

func fromEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

func intFromEnv(key string, fallback int) int {
	val, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	value, err := strconv.Atoi(val)
	if err != nil {
		panic(err)
	}
	return value
}

func durationFromEnv(key string, fallback time.Duration) time.Duration {
	val, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	value, err := time.ParseDuration(val)
	if err != nil {
		panic(err)
	}
	return value
}
