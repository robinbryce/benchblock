package collect

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	_ "github.com/mattn/go-sqlite3"
	"github.com/robinbryce/blockbench/bbencheth/client"
)

type BlockDB struct {
	db          *sql.DB
	insertBlock *sql.Stmt
	timeScale   time.Duration
}

const (
	// results collection compatible with chainhammer analysis scripts
	CreateTableStmt = `CREATE TABLE IF NOT EXISTS blocks(
 	   	blocknumber INTEGER UNIQUE
 	   	,timestamp DECIMAL
 	   	,size INTEGER
 	   	,gasUsed INTEGER
 	   	,gasLimit INTEGER
 	   	,txcount INTEGER
		,hash TEXT
		,parentHash TEXT
		,extra TEXT
		)`
	InsertStmt = `INSERT INTO blocks(
			blocknumber,timestamp,size,
			gasUsed,gasLimit,txcount,extra)
			VALUES(?,?,?,?,?,?,?)`
)

func NewBlockDB(dataSourceName string, share bool) (*BlockDB, error) {

	var err error

	if dataSourceName == "" {
		return nil, nil
	}

	// Don't allow re-use of a db from a previous run
	if !share && dataSourceName != ":memory:" {

		u, err := url.Parse(dataSourceName)
		if err != nil {
			return nil, err
		}

		var filename string
		switch {
		case u.Scheme == "":
			filename = u.Path
		case u.Scheme == "file":
			filename = u.Opaque
		default:
			filename = u.Path
		}
		if u.Scheme == "" {
			filename = u.Path
		}
		if _, err := os.Stat(filename); err == nil {
			return nil, fmt.Errorf("sqlite3 dsn '%s' exists, updating not supported - move or delete the file please", filename)
		}
	}

	bdb := &BlockDB{
		// We do (block.time * timeScale) to get nanoseconds then record millis.
		// For quorum, this derives to (block.time * 1) / 1000. Etherum main
		// chain is timeScale == time.Seconds
		timeScale: time.Nanosecond,
	}
	if bdb.db, err = sql.Open("sqlite3", dataSourceName); err != nil {
		fmt.Printf("failed to open: %s\n", dataSourceName)
		return nil, err
	}

	// Create the table if it does not exist
	s, err := bdb.db.Prepare(CreateTableStmt)
	if err != nil {
		return nil, err
	}

	_, err = s.Exec()
	if err != nil {
		return nil, err
	}

	// prepare the insert statement
	bdb.insertBlock, err = bdb.db.Prepare(InsertStmt)

	if err != nil {
		return nil, err
	}

	return bdb, nil
}

// Insert a block record into the database. Not transactional
func (bdb *BlockDB) Insert(
	block *types.Block, header *types.Header) error {

	// Always record the timestamp exactly as we get it to avoid un-intentional 'lossyness'
	_, err := bdb.insertBlock.Exec(
		header.Number.Int64(), header.Time, block.Size(),
		header.GasUsed, header.GasLimit, len(block.Transactions()),
		hex.EncodeToString(header.Extra),
	)
	return err
}

func GetBlocks(ethEndpoint, dbname string, dbshare bool, retries int, start, end int64) error {

	var err error

	eth, err := client.NewEthClient(ethEndpoint)
	if err != nil {
		return fmt.Errorf("creating eth client: %w", err)
	}

	db, err := NewBlockDB(dbname, dbshare) // returns nil for dbdsn == ""
	if err != nil {
		return err
	}

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

		block, err = client.GetBlockByNumber(context.TODO(), eth, retries, n)
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
