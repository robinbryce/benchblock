package loadtool

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
)

type BlockDB struct {
	db          *sql.DB
	insertBlock *sql.Stmt
	timeScale   time.Duration
}

const (
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
