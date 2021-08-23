# blockbench

Tools to explore the performance characteristics of different etherum/quorum
private network deployments. Read "bench" as in work/bench rather than
bench/mark.

Current features

* Single command to deploy a network based on ibft, raft or rrr[^1]
* Loadtesting tool with output to chainhammer compatible db format
* Single command performance graph generation & reports (markdown, jupytext, papermill)
* VScode remote debug support for docker-compose hosted nodes running from volume mounted source checkout.

Please note the jupyter charting support owes much to the
[chainhammer](https://github.com/drandreaskrueger/chainhammer/blob/master/README.md)
project, which also supports a broader range of ledgers. Please do consider
whether that project better suites your needs.

[^1]: A pre-alpha implementation of this [paper](https://arxiv.org/pdf/1804.07391.pdf) for go-ethereum. Testing and development of which was a key motivation for this project.

# Quick example

Having completed [Setup](#Setup), these are the steps to deploy and load test a 

```bash
cd ~/workspace
bbench raft -n 5 raft5
cd raft5
docker-compose up -d
docker-compose logs -f node1
# watch the log for a bit to see that raft a peercount=4 in the logs

# Run the load generation tool (from source for now)
cd ../blockbench/go/loadtool
go run main.go ethload -e http://127.0.0.1:8300/ -t 5 -a 3 --dbsource ~/workspace/raft5/raft5.db

cd ~/workspace/raft5
bbench jpycfg .
# Note: still ironing out some rough edges with the chart generation
bbench jpyrender .
```

# Setup

Python >= 3.8 and docker (with docker-compose) is assumed. Currently, a go ~1.15
development setup is required to run the load generation tool. If there is any
interest, binary releases will be provided.

Convenience install script for linux amd64 (also fine for wsl2)
```sh
echo "Installing for linux_amd64"
mkdir ~/workspace && cd ~/workspace

# Install yq (jq for yaml)
YQ_VERSION=v4.12.0

# NOTICE: change this if not on linux x86_64
YQ_BINARY=yq_linux_amd64

wget https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/${YQ_BINARY}.tar.gz -O - |\
  tar xz && mv ${YQ_BINARY} ~/.local/bin/yq

# To take advantage of vscode remote debug (to docker-compose node) support
# higher versions should work fine. 2.6.0 is the earliest expected to work (due
# to reliance on dns resolution feature)
git clone https://github.com/ConsenSys/quorum.git
$(cd quorum && git checkout v21.4.1)

# Install go-task and alias the main entry point
curl -sL https://git.io/tusk | bash -s -- -b ~/.local/bin latest

git clone https://github.com/robinbryce/blockbench.git
alias bbench='tusk -qf ~/workspace/blockbench/tusk.yml'
```

Please see [go-tusk](https://github.com/rliebz/tusk#readme), [yq](https://github.com/mikefarah/yq/blob/master/README.md) for up to installation details and information for other platforms.

The [go-quorum](https://github.com/ConsenSys/quorum.git) clone clone is needed to take advantage of  compose remote debug  configuration [^2], but otherwise can be ommitted