package load

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/robinbryce/blockbench/bbencheth/client"
	"github.com/robinbryce/blockbench/bbencheth/collect"
	"github.com/robinbryce/blockbench/bbencheth/root"
)

const (
	GetSetAddABI = `[ { "constant": false, "inputs": [ { "internalType": "uint256", "name": "x", "type": "uint256" } ], "name": "add", "outputs": [], "payable": false, "stateMutability": "nonpayable", "type": "function" }, { "constant": false, "inputs": [ { "internalType": "uint256", "name": "x", "type": "uint256" } ], "name": "set", "outputs": [], "payable": false, "stateMutability": "nonpayable", "type": "function" }, { "constant": true, "inputs": [], "name": "get", "outputs": [ { "internalType": "uint256", "name": "retVal", "type": "uint256" } ], "payable": false, "stateMutability": "view", "type": "function" } ]`
	GetSetAddBin = "0x608060405234801561001057600080fd5b50610126806100206000396000f3fe6080604052348015600f57600080fd5b50600436106059576000357c0100000000000000000000000000000000000000000000000000000000900480631003e2d214605e57806360fe47b11460895780636d4ce63c1460b4575b600080fd5b608760048036036020811015607257600080fd5b810190808035906020019092919050505060d0565b005b60b260048036036020811015609d57600080fd5b810190808035906020019092919050505060de565b005b60ba60e8565b6040518082815260200191505060405180910390f35b806000540160008190555050565b8060008190555050565b6000805490509056fea265627a7a72315820c8bd9d7613946c0a0455d5dcd9528916cebe6d6599909a4b2527a8252b40d20564736f6c634300050b0032"
)

var (
	big0 = big.NewInt(0)
	big1 = big.NewInt(1)
)

type ContractTransactor interface {
	Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error)
}

type LoaderOption func(*Loader)

// WithProgress enables a progress bar for the Loader. Pass a nil progress
// to use the internal default. Pass a previously created instance to share a
// progress meter. To disable progress entirely, just don't supply this option.
func WithProgress(pb *client.TransactionProgress) LoaderOption {
	return func(lo *Loader) {
		// If the caller provided a pre configured progress meter, use it as is.
		// Otherwise create one just for tracking mined transactions.
		if pb == nil {
			pb = client.NewTransactionProgress(lo.loadCfg.NumTransactions, client.WithIssuedProgress(), client.WithMinedProgress())
		}
		lo.pb = pb
	}
}

// WithCollector enables the transaction collector
func WithCollector(c *collect.Collector) LoaderOption {
	return func(lo *Loader) {
		lo.collector = c
	}
}

const (
	ConfigName = "load"
)

type Config struct {
	client.AccountConfig
	collect.ConfigTransactions

	TPS         int           `mapstructure:"tps"`
	CollectRate time.Duration `mapstructure:"collect_rate"`

	// By default we assume the number of nodes = the number of threads. To
	// create multiple clients per node set Nodes to the node count and set
	// Threads to Nodes * DesiredClients
	Nodes int `mapstructure:"nodes"`

	// SingleNode is set if all transactions should be issued to the same
	// node. This isolates the load for transaction submission to a single node.
	// We then get an idea of how efficiently the remaining nodes reach
	// consensus on those transactions. Assuming that node is at least able to
	// effectively gossip those transactions. If there is a significant
	// difference with and without this set then the nodes are likely just under
	// resourced.
	SingleNode bool `mapstructure:"singlenode"`

	// If true, confirm every transaction in a batch before doing the next batch.
	CheckReceipts bool `mapstructure:"check_receipts"`

	TesseraEndpoint string `mapstructure:"tesera"`
	// If staticnodes is provided, Nodes hosts are read from the file. Its
	// assumed to be the same format as static-nodes.json. Only the hostname
	// (or IP) field of the url is significant. The port is set seperately
	// (baseport) and must be the same for all nodes.
	StaticNodes     string `mapstructure:"staticnodes"`
	BaseTesseraPort int    `mapstructure:"basetesseraport"`

	ExpectedLatency time.Duration `mapstructure:"expected_latency"`

	DeployGasLimit uint64 `mapstructure:"deploy_gaslimit"`
	DeployKey      string `mapstructure:"deploy_key"` // needs to have funds even for quorum, used to deploy contract
	RunOne         bool   `mapstructure:"run_one"`
}

func NewConfigLoader() Config {
	cfg := Config{}
	cfg.SetDefaults()
	return cfg
}

func (cfg *Config) SetDefaults() {
	cfg.Threads = 12
	cfg.ThreadAccounts = 6
	cfg.NumTransactions = 5000
	cfg.TPS = 221
	cfg.GasLimit = 60000
	cfg.PrivateFor = ""
	cfg.SingleNode = false
	cfg.CheckReceipts = false
	cfg.StaticNodes = ""
	cfg.BaseTesseraPort = 0
	cfg.TesseraEndpoint = ""
	cfg.ExpectedLatency = 10 * time.Second
	cfg.DeployGasLimit = 600000
	cfg.DeployKey = ""
	cfg.RunOne = false
	cfg.CollectRate = 10 * time.Second
}

// Loader runs a load of 'add' contract calls, according to the load
// configuration, to the standard get/set/add example solidity contract.
type Loader struct {
	rootCfg       *root.Config
	loadCfg       *Config
	collector     *collect.Collector
	ConfigFileDir string

	pb *client.TransactionProgress

	limiter *time.Ticker
	// One AccountSet per thread
	accounts []client.AccountSet
	// One connection per thread
	ethC    []*client.Client
	ethCUrl []string

	contract ContractTransactor
	address  common.Address
}

func NewLoader(ctx context.Context, configFileDir string, r root.Runner, opts ...LoaderOption) (Loader, error) {

	var err error

	a := Loader{
		ConfigFileDir: configFileDir,
		rootCfg:       r.GetParent().GetConfig().(*root.Config),
		loadCfg:       r.GetParent().GetNamedConfig(r.GetName()).(*Config),
	}

	// NumTransactions needs to be adjusted before processing the options (so the progress options can be correctly applied)
	if delta := a.loadCfg.TruncateTargetTransactions(); delta != 0 {
		fmt.Printf(
			"adjusted target number of transactions from %d to %d\n",
			a.loadCfg.NumTransactions+delta, a.loadCfg.NumTransactions)
	}

	for _, opt := range opts {
		opt(&a)
	}

	// Progress is disabled entirely if the WithProgress option is not
	// supplied. This is most cleanly accomplished with NoOp progress
	if a.pb == nil {
		a.pb = client.NewTransactionProgress(a.loadCfg.NumTransactions)
	}

	a.limiter = time.NewTicker(time.Second / time.Duration(a.loadCfg.TPS))

	var tx *types.Transaction

	var deployKey *ecdsa.PrivateKey
	parsed, err := abi.JSON(strings.NewReader(GetSetAddABI))
	if err != nil {
		return Loader{}, err
	}

	if a.loadCfg.DeployKey != "" {
		deployKey, err = crypto.HexToECDSA(a.loadCfg.DeployKey)
		if err != nil {
			return Loader{}, err
		}
	}
	if deployKey == nil {
		// This will likely fail as normal quorum requires balance to deploy
		// event tho gasprice is 0
		deployKey, err = crypto.GenerateKey()
		if err != nil {
			return Loader{}, err
		}
	}

	switch {
	case a.rootCfg.EthEndpoint != "":
		if err = a.clientsFromEthEndpoint(ctx); err != nil {
			return Loader{}, err
		}
	case a.loadCfg.StaticNodes != "":
		if err = a.clientsFromStaticNodes(ctx); err != nil {
			return Loader{}, err
		}
	default:
		return Loader{}, fmt.Errorf("you must provide either --ethendpoint or --staticnodes")
	}

	deployAuth := bind.NewKeyedTransactor(deployKey)

	deployAuth.GasLimit = uint64(a.loadCfg.DeployGasLimit)
	deployAuth.GasPrice = big0
	a.address, tx, a.contract, err = bind.DeployContract(
		deployAuth, parsed, common.FromHex(GetSetAddBin), a.ethC[0])
	if err != nil {
		return Loader{}, err
	}
	if ok := client.CheckReceipt(a.ethC[0].Client, tx, a.rootCfg.Retries, a.loadCfg.ExpectedLatency); !ok {
		return Loader{}, fmt.Errorf("failed to deploy contract")
	}

	// num-tx / num-threads

	return a, nil
}

func (a *Loader) Run() {

	var wg sync.WaitGroup

	for i := 0; i < a.loadCfg.Threads; i++ {
		wg.Add(1)
		clientId, addr := fmt.Sprintf("client-%d", i), a.ethCUrl[i]
		fmt.Printf("client thread: %s, node addr:%s\n", clientId, addr)
		go a.adder(a.ethC[i].Client, &wg, clientId, i)
	}

	if a.collector != nil {
		wg.Add(1)
		go a.collector.Collect(a.ethC[0], &wg, fmt.Sprintf("client-%d", 0), 0)
	}
	wg.Wait()
	if a.pb.IsEnabled() {
		fmt.Printf("sent: %d, mined: %d\n", a.pb.CurrentIssued(), a.pb.CurrentMined())
	}
}

// RunOne is provided for dignostic purposes. It issues a single transaction
// using the first account in the first account set for the provided
// configuration.
func (lo *Loader) RunOne() error {

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
	tx, err = lo.contract.Transact(auth, "add", big.NewInt(3))
	if err != nil {
		return err
	}
	if ok := client.CheckReceipt(ethC.Client, tx, lo.rootCfg.Retries, lo.rootCfg.ClientTimeout); !ok {
		return fmt.Errorf("transaction %s failed or not completed in %v", tx.Hash().Hex(), lo.rootCfg.ClientTimeout)
	}
	return nil
}

func (lo *Loader) adder(ethC *ethclient.Client, wg *sync.WaitGroup, banner string, ias int) {

	defer wg.Done()

	var tx *types.Transaction

	// Note: NumTransactions is adjusted by TruncateTargetTransactions so
	// everything works out as whole numbers. And so that
	//  NumTransactions >= lo.cfg.NumThreads * lo.cfg.AccountsPerThread
	numBatches := lo.loadCfg.NumTransactions / (lo.loadCfg.Threads * lo.loadCfg.ThreadAccounts)

	// Each batch issues on tx per account. The batching is only worth it if
	// CheckBatchReciepts is true (it cleans up the picture by reducing the eth
	// rpc load  on the node)
	batch := make([]*types.Transaction, lo.loadCfg.ThreadAccounts)

	var err error

	// First, initialise the nonces for the transactors in the AccountSet, and also set the ctx in the auth's
	for i := 0; i < lo.loadCfg.ThreadAccounts; i++ {
		var nonce uint64
		ctx, cancel := context.WithTimeout(context.Background(), lo.rootCfg.ClientTimeout)
		nonce, err = ethC.PendingNonceAt(ctx, lo.accounts[ias].Wallets[i])
		cancel()
		if err != nil {
			fmt.Printf("terminating client for %s. error initialsing nonce: %v\n", lo.ethCUrl[ias], err)
		}

		lo.accounts[ias].Auth[i].Nonce = big.NewInt(int64(nonce))
	}

	for r := 0; r < numBatches; r++ {

		if !lo.pb.IsEnabled() {
			fmt.Printf("%s: batch %d, node %s\n", banner, r, lo.ethCUrl[ias])
		}

		for i := 0; i < lo.loadCfg.ThreadAccounts; i++ {
			if lo.limiter != nil {
				<-lo.limiter.C
			}

			// Set the ctx for the auth
			_, cancel := lo.accounts[ias].WithTimeout(context.Background(), lo.rootCfg.ClientTimeout, i)
			tx, err = lo.contract.Transact(lo.accounts[ias].Auth[i], "add", big.NewInt(2))
			cancel()
			if err != nil {
				fmt.Printf("terminating client for %s. error from transact: %v\n", lo.ethCUrl[ias], err)
				return
			}
			lo.pb.IssuedIncrement()

			lo.accounts[ias].IncNonce(i)
			batch[i] = tx
		}
		if lo.loadCfg.CheckReceipts {
			for i := 0; i < lo.loadCfg.ThreadAccounts; i++ {
				if ok := client.CheckReceipt(ethC, batch[i], lo.rootCfg.Retries, lo.loadCfg.ExpectedLatency); !ok {
					fmt.Printf("terminating client for %s. no valid receipt found for tx: %s\n",
						lo.ethCUrl[ias], tx.Hash().Hex())
				}
			}
		}
	}
}

func (a *Loader) resolveHost(host string) (string, error) {
	if !a.rootCfg.ResolveHosts {
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

func (a *Loader) clientsFromEthEndpoint(ctx context.Context) error {

	if a.rootCfg.EthEndpoint == "" {
		return fmt.Errorf("ethendpoint is empty")
	}

	nodes := a.loadCfg.Nodes
	if nodes == 0 {
		nodes = a.loadCfg.Threads
	}

	a.accounts = make([]client.AccountSet, a.loadCfg.Threads)
	a.ethC = make([]*client.Client, a.loadCfg.Threads)
	a.ethCUrl = make([]string, a.loadCfg.Threads)

	qu, err := url.Parse(a.rootCfg.EthEndpoint)
	if err != nil {
		return err
	}

	quHostname, err := a.resolveHost(qu.Hostname())
	if err != nil {
		return err
	}

	baseQuorumPort := a.rootCfg.BasePort
	if baseQuorumPort == 0 {
		baseQuorumPort, err = strconv.Atoi(qu.Port())
		if err != nil {
			return err
		}
	}

	var tu *url.URL
	var tuHostname string
	var baseTesseraPort int
	if a.loadCfg.TesseraEndpoint != "" {
		tu, err = url.Parse(a.loadCfg.TesseraEndpoint)
		if err != nil {
			return err
		}

		tuHostname, err = a.resolveHost(tu.Hostname())
		if err != nil {
			return err
		}

		baseTesseraPort, err = strconv.Atoi(tu.Port())
		if err != nil {
			return err
		}
	}

	for i := 0; i < a.loadCfg.Threads; i++ {

		qu.Host = fmt.Sprintf("%s:%d", quHostname, baseQuorumPort)
		if !a.loadCfg.SingleNode && nodes > 1 {
			qu.Host = fmt.Sprintf("%s:%d", quHostname, baseQuorumPort+(i%(nodes)))
		}
		var tuEndpoint string
		if tu != nil && nodes > 1 {
			tu.Host = fmt.Sprintf("%s:%d", tuHostname, baseTesseraPort+(i%(nodes)))
			tuEndpoint = tu.String()
		}

		a.ethC[i], err = client.NewClient(qu.String(), tuEndpoint, a.rootCfg.ClientTimeout)
		if err != nil {
			return err
		}
		a.ethCUrl[i] = qu.String()

		fmt.Printf("building account set for client[%d]: %s\n", i, qu.String())

		a.accounts[i], err = client.NewAccountSet(ctx, a.ethC[i].Client, &a.loadCfg.AccountConfig, a.loadCfg.ThreadAccounts)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *Loader) clientsFromStaticNodes(ctx context.Context) error {
	if a.loadCfg.StaticNodes == "" {
		return fmt.Errorf("staticnodes is empty")
	}

	fileName := filepath.Join(a.ConfigFileDir, a.loadCfg.StaticNodes)
	var staticNodes []string
	if err := common.LoadJSON(fileName, &staticNodes); err != nil {
		return fmt.Errorf("loading file `%s': %v", fileName, err)
	}

	nodes := a.loadCfg.Nodes
	if nodes == 0 {
		nodes = a.loadCfg.Threads
	}

	if nodes > len(staticNodes) {
		return fmt.Errorf(
			"to few nodes in %s. need %d, have %d", a.loadCfg.StaticNodes, nodes, len(staticNodes))
	}

	a.accounts = make([]client.AccountSet, a.loadCfg.Threads)
	a.ethC = make([]*client.Client, a.loadCfg.Threads)
	a.ethCUrl = make([]string, a.loadCfg.Threads)

	quorumPort := a.rootCfg.BasePort
	if quorumPort == 0 {
		quorumPort = 8545
	}
	tesseraPort := a.loadCfg.BaseTesseraPort
	if tesseraPort == 0 {
		tesseraPort = 50000
	}

	var err error
	qurls := make([]url.URL, len(staticNodes))
	turls := make([]url.URL, len(staticNodes))
	for i := 0; i < len(staticNodes); i++ {
		qu, err := url.Parse(staticNodes[i])
		if err != nil {
			return err
		}

		// Ignore the port in the file. If its an actual static-nodes.json it
		// will be the p2p port
		parts := strings.Split(qu.Host, ":")
		parts[len(parts)-1] = strconv.Itoa(quorumPort)
		qurls[i].Scheme = "http"
		parts[0], err = a.resolveHost(parts[0])
		if err != nil {
			return err
		}
		qurls[i].Host = strings.Join(parts, ":")

		parts[len(parts)-1] = strconv.Itoa(tesseraPort)
		turls[i].Scheme = "http"
		turls[i].Host = strings.Join(parts, ":")
	}

	for i := 0; i < a.loadCfg.Threads; i++ {

		qu, tu := qurls[0], turls[0]

		if !a.loadCfg.SingleNode && nodes > 1 {
			qu = qurls[i%(nodes)]
			tu = turls[i%(nodes)]
		}

		quEndpoint := qu.String()
		tuEndpoint := ""
		if a.loadCfg.BaseTesseraPort != 0 {
			// TODO: Better handling of tessera
			tuEndpoint = tu.String()
		}

		a.ethC[i], err = client.NewClient(quEndpoint, tuEndpoint, a.rootCfg.ClientTimeout)
		if err != nil {
			return err
		}
		a.ethCUrl[i] = quEndpoint

		fmt.Printf("building account set for client[%d]: %s\n", i, quEndpoint)

		a.accounts[i], err = client.NewAccountSet(ctx, a.ethC[i].Client, &a.loadCfg.AccountConfig, a.loadCfg.ThreadAccounts)
		if err != nil {
			return err
		}
	}
	return nil
}
