package client

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	big1 = big.NewInt(1)
)

type AccountConfig struct {
	GasLimit   uint64
	PrivateFor string
	MangeNonce bool
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

func NewAccountSet(ctx context.Context, ethC *ethclient.Client, cfg *AccountConfig, n int) (AccountSet, error) {

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

		if !cfg.MangeNonce {
			continue
		}
		if nonce, err = ethC.PendingNonceAt(ctx, a.Wallets[i]); err != nil {
			return AccountSet{}, err
		}
		a.Auth[i].Nonce = big.NewInt(int64(nonce))
	}
	return a, nil
}
