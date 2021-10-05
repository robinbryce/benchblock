module github.com/robinbryce/blockbench/loadtool

go 1.15

// Upstream go-quorum is in a bit of a pickle due to their addoption of
// callendar verions and go's insistence on MAJOR semver matching the declared
// path of the module *if it is > 1*

// Using pseudo versions is the only way I can see to solve this for now. To
// generate the pseudo version clone github.com/ConesenSys/quorum and checkout
// the tag you want. Then run:
//
//     TZ=UTC git --no-pager show --quiet --abbrev=12 --date='format-local:%Y%m%d%H%M%S' --format="%cd-%h"
//
// CURRENT GO_QUORUM_VERSION=v21.7.1
replace (
	// etcd v3.3.13-quorum197 =>
	github.com/coreos/etcd => github.com/Consensys/etcd v0.0.0-20210205143158-25bddae346fa
	github.com/ethereum/go-ethereum => github.com/ConsenSys/quorum v0.0.0-20210819085930-d5ef77cafd90
	github.com/ethereum/go-ethereum/crypto/secp256k1 => github.com/ConsenSys/quorum/crypto/secp256k1 v0.0.0-20210819085930-d5ef77cafd90
)

require (
	github.com/ethereum/go-ethereum v0.0.0-00010101000000-000000000000
	github.com/ethereum/go-ethereum/crypto/secp256k1 v0.0.0
	github.com/mattn/go-sqlite3 v1.14.8
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.9.0
	github.com/stretchr/testify v1.7.0
	github.com/vbauerster/mpb/v7 v7.1.5
)
