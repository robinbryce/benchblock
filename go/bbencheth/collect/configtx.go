package collect

// ConfigTransactions defines the transaction load pattern. Multiple commands
// need this. The total number of transactions issued for the run is calculated
// from this config
type ConfigTransactions struct {
	// Each thread issues transactions to its own client connection. If
	// SingleNode is set, then those connections are all to the same node.
	// Otherwise each client connection connects to a different node.
	Threads int `mapstructure:"THREADS"`

	// How many accounts to use on each thread. defaults to 5
	ThreadAccounts int `mapstructure:"THREADACCOUNTS"`

	// The target total number of transactions. At least NumThreads * AccountsPerThread tx are issued.
	NumTransactions int `mapstructure:"TRANSACTIONS"`
}

// TruncateTargetTransactions conditions the requested NumTransactions so that
// all threads serve the same number of transactions. Each thread will serve at
// least one tx.
func (cfg *ConfigTransactions) TruncateTargetTransactions() int {
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
