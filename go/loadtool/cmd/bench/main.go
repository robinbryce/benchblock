package main

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/urfave/cli.v1"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/robinbryce/blockbench/loadtool"
)

var (
	// Git information set by linker when building with ci.go.
	gitCommit string
	gitDate   string
	app       = &cli.App{
		Name:        filepath.Base(os.Args[0]),
		Usage:       "RobustRoundRobin consensus tool for ConsenSys/quorum",
		Version:     params.VersionWithCommit(gitCommit, gitDate),
		Writer:      os.Stdout,
		HideVersion: true,
	}
)

func init() {
	// Set up the CLI app.

	app.CommandNotFound = func(ctx *cli.Context, cmd string) {
		fmt.Fprintf(os.Stderr, "No such command: %s\n", cmd)
		os.Exit(1)
	}

	// Add subcommands.
	app.Commands = []cli.Command{
		loadGenCmd,
		getBlocksCmd,
	}
}

var getBlocksCmd = cli.Command{
	Name:   "getblocks",
	Usage:  "Get a block from a live node and inspect the rrr details in the header",
	Action: getBlocks,
	Flags: []cli.Flag{
		cli.StringFlag{Name: "endpoint", Usage: "http(s)://host:port to connect to"},
		cli.Int64Flag{Name: "start", Usage: "first to GET"},
		cli.Int64Flag{Name: "end", Usage: "last to GET"},
		cli.IntFlag{Name: "retries", Usage: "limit the number of retries for the getBlockByNumber. Otherwise retry forever with exponential backoff"},
		cli.BoolFlag{Name: "dbshare", Usage: `the db records are always
		inserted. by default we do not allow re-use of a db. set this flag to
		enable reuse. you are responsible for using non over lapping start & end
		blocks.`},
		cli.StringFlag{Name: "dbname", Usage: "sqlitedb datasourcename. usualy just the plain filename eg 'blocks.db'"},
	},
}

func getBlocks(ctx *cli.Context) error {

	var err error

	endpoint := ctx.String("endpoint")
	eth, err := loadtool.NewEthClient(endpoint)
	if err != nil {
		return fmt.Errorf("creating eth client: %w", err)
	}

	db, err := loadtool.NewBlockDB(ctx.String("dbname"), ctx.Bool("dbshare")) // returns nil for dbdsn == ""
	if err != nil {
		return err
	}

	start := ctx.Int64("start")
	end := ctx.Int64("end")
	if end < start {
		return fmt.Errorf("start cant be greater than end")
	}

	tprev := int64(-1)
	if start >= 1 {

		block, err := eth.BlockByNumber(context.TODO(), new(big.Int).SetInt64(start-1))
		if err != nil {
			return fmt.Errorf("eth_blockByNumber %d: %w", start-1, err)
		}
		tprev = int64(block.Header().Time)
	}

	var block *types.Block

	for n := start; n <= end; n++ {

		block, err = loadtool.GetBlockByNumber(eth, ctx.Int("retries"), n)
		if err != nil {
			return fmt.Errorf("eth_blockByNumberd: %w", err)
		}

		h := block.Header()

		if db != nil {
			if err := db.Insert(block, h); err != nil {
				println(fmt.Errorf("inserting block %v: %w", h.Number, err).Error())
			}
		}

		// print out block number, sealer, endorer1 ... endorsern
		delta := "NaN"
		if tprev != -1 {
			delta = fmt.Sprintf("%d", int64(h.Time)-tprev)
		}
		tprev = int64(h.Time)

		s := int64(h.Time)
		t := time.Unix(s, 0).Format(time.RFC3339)
		fmt.Printf("%d %s %s", h.Number.Int64(), delta, t)
		fmt.Println("")
	}

	return nil
}

var loadGenCmd = cli.Command{
	Name:   "load",
	Usage:  "issue transactions at the configured rate to the configured range of ports in paralel. optionally gather reciepts.",
	Action: getBlocks,
	Flags: []cli.Flag{
		cli.StringFlag{Name: "endpoint", Usage: "http(s)://host:port to connect to"},
		cli.Int64Flag{Name: "start", Usage: "first to GET"},
		cli.Int64Flag{Name: "end", Usage: "last to GET"},
		cli.IntFlag{Name: "retries", Usage: "limit the number of retries for the getBlockByNumber. Otherwise retry forever with exponential backoff"},
		cli.BoolFlag{Name: "dbshare", Usage: `the db records are always
		inserted. by default we do not allow re-use of a db. set this flag to
		enable reuse. you are responsible for using non over lapping start & end
		blocks.`},
		cli.StringFlag{Name: "dbname", Usage: "sqlitedb datasourcename. usualy just the plain filename eg 'blocks.db'"},
	},
}

func loadGen(ctx *cli.Context) error {

	println("loadGen TBD")
	return nil
}

func main() {
	exit(app.Run(os.Args))
}

func exit(err interface{}) {
	if err == nil {
		os.Exit(0)
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
