package loader

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

// ethereum rpc requests
func GetBlockByNumber(ctx context.Context, eth *ethclient.Client, retries int, blockNumber int64) (*types.Block, error) {

	bigN := new(big.Int).SetInt64(blockNumber)

	var err error // will return the last error or nil
	var block *types.Block

	for i := 0; retries == 0 || i < retries; i++ {

		block, err = eth.BlockByNumber(ctx, bigN)
		if err == nil {
			return block, nil
		}
		if errors.Is(err, context.Canceled) {
			return nil, err
		}
		time.Sleep(backoffDuration(i))
	}
	return block, err
}

func CheckReceipt(
	ethC *ethclient.Client, tx *types.Transaction, retries int, expectedLatency time.Duration) bool {

	// start := time.Now()
	// fmt.Printf("checkreceipt: %s\n", tx.Hash().Hex())
	for i := 0; i < retries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), expectedLatency)
		r, err := ethC.TransactionReceipt(ctx, tx.Hash())
		cancel()
		if r == nil || err != nil {
			// fmt.Printf("trying for %v, backoff & retry: err=%v\n", time.Since(start), err)
			time.Sleep(backoffDuration(i))
			continue
		}
		if r.Status == 1 {
			return true
		}
		return false
	}
	return false
}

// derived from https://blog.gopheracademy.com/advent-2014/backoff/
var backoffms = []int{0, 500, 500, 1000, 1000, 2000, 2000, 4000, 4000, 10000, 10000, 10000, 10000, 10000, 10000}

func backoffDuration(nth int) time.Duration {
	if nth >= len(backoffms) {
		nth = len(backoffms) - 1
	}
	return time.Duration(jitter(backoffms[nth])) * time.Millisecond
}

func jitter(ms int) int {
	if ms == 0 {
		return 0
	}
	return ms/2 + rand.Intn(ms)
}

func NewEthClient(ethEndpoint string) (*ethclient.Client, error) {

	ethRPC, err := rpc.DialHTTPWithClient(ethEndpoint, &http.Client{Timeout: time.Second * 10})
	if err != nil {
		return nil, err
	}
	ethClient := ethclient.NewClient(ethRPC)
	if ethClient == nil {
		return nil, fmt.Errorf("failed creating ethclient")
	}

	return ethClient, nil
}

func NewTransactor(ethEndpoint, tesseraEndpoint string, clientTimeout time.Duration) (*ethclient.Client, error) {
	ethC, err := NewClient(ethEndpoint, tesseraEndpoint, clientTimeout)
	return ethC.Client, err
}

// Client makes both the ethclient and the underlying rpc.Client available on the same struct
type Client struct {
	*ethclient.Client
	RPC *rpc.Client
}

func NewClient(
	ethEndpoint, tesseraEndpoint string, clientTimeout time.Duration,
) (*Client, error) {

	c := &Client{}

	ethRPC, err := rpc.DialHTTPWithClient(ethEndpoint, &http.Client{Timeout: clientTimeout})
	if err != nil {
		return nil, err
	}
	c.RPC = ethRPC
	ethClient := ethclient.NewClient(ethRPC)
	if ethClient == nil {
		return nil, fmt.Errorf("failed creating ethclient")
	}
	c.Client = ethClient

	if tesseraEndpoint != "" {
		ethClient, err = ethClient.WithPrivateTransactionManager(tesseraEndpoint)
		c.Client = ethClient
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}
