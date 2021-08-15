package loadtool

import (
	"context"
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
func GetBlockByNumber(eth *ethclient.Client, retries int, blockNumber int64) (*types.Block, error) {

	bigN := new(big.Int).SetInt64(blockNumber)

	var err error // will return the last error or nil
	var block *types.Block

	for i := 0; retries == 0 || i < retries; i++ {

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		block, err = eth.BlockByNumber(ctx, bigN)
		if err == nil {
			cancel()
			return block, nil
		}
		cancel()
		time.Sleep(backoffDuration(i))
	}
	return block, err
}

func CheckReceipt(
	ethC *ethclient.Client, tx *types.Transaction, retries int, expectedLatency time.Duration) bool {

	for i := 0; i < retries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), expectedLatency)
		r, err := ethC.TransactionReceipt(ctx, tx.Hash())
		cancel()
		if r == nil || err != nil {
			// fmt.Printf("backoff & retry: err=%v\n", err)
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
var backoffms = []int{0, 0, 10, 10, 100, 100, 500, 500, 3000, 3000, 5000, 5000, 8000, 8000, 10000, 10000}

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

	ethRPC, err := rpc.DialHTTPWithClient(ethEndpoint, &http.Client{Timeout: clientTimeout})
	if err != nil {
		return nil, err
	}
	ethClient := ethclient.NewClient(ethRPC)
	if ethClient == nil {
		return nil, fmt.Errorf("failed creating ethclient")
	}

	if tesseraEndpoint != "" {
		ethClient, err = ethClient.WithPrivateTransactionManager(tesseraEndpoint)
		if err != nil {
			return nil, err
		}
	}
	return ethClient, nil
}
