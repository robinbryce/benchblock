# benchblock

[![build](https://github.com/robinbryce/benchblock/actions/workflows/build-images.yaml/badge.svg)](https://github.com/robinbryce/benchblock/actions/workflows/build-images.yaml)

[![Load test one configuration for each consensus alg](https://github.com/robinbryce/benchblock/actions/workflows/loadtest-each-consensus.yaml/badge.svg)](https://github.com/robinbryce/benchblock/actions/workflows/loadtest-each-consensus.yaml)


A tool to create and configure ethereum/quorum networks. For deployment with
compose and with kubernetes. Emphasis is given to fast creation and
re-configuration of different shaped networks.

Why benchblock ? A rubber bench block is typically placed on a work bench to
absorb the shock of working metal or wire with a hammer.  Read "bench" as in
work/bench rather than bench/mark.

Features

* Create kustomize manifests for ibft, raft or rrr[^1] networks
* Create docker-compose setup for ibft, raft or rrr[^1] networks
* Loadtesting tool with output to chainhammer compatible db format
* GitHub action automation example using kubernetes job to run in-cluster loadtest
* Graph generation & reports (markdown, jupytext, papermill).
  ![raft-example](examples/raft5-compose-plot-combined.png)

  (Note: Axis labels are only visible in the github *light* theme.)
* Selection of canned configurations for setting up ibft, raft or rrr[^1].
* Discovery enabled networks with bootnodes (rrr only for now, ibft and raft use static-nodes.json)
* Compose configuration facilitates vscode remote debugging of nodes
* Compose configurations support running geth/quorum nodes from sources (go run on volume mounted sources)

# Quick example

Having completed [Setup](#Setup), (including installing the loadtest tool)
these are the steps to deploy and load test a raft 5 node network

```bash

bbake new -p 5 raft5 raft
cd raft5

docker-compose up -d
docker-compose logs -f node1
# watch the log for a bit to see that raft a peercount=4 in the logs

bbeth http://127.0.0.1:8300/ load -t 5 -a 3 --dbsource raft5.db

bbake jpycfg .
bbake jpyrender .
```

Then open standard-plots.html (it is self contained)

# Jupyter

```sh
# assumes `Quick example` (above)
env/bin/jupyter notebook --ip=127.0.0.1
```

Follow the instructions in the log to access the environment in your browser

# Kubernetes deployment

## raft

```sh
bbake new -p 8 -k raftk8 raft
kustomize build raftk8/raft | kubectl apply -f -
```

## ibft

```sh
bbake new -p 8 -k ibftk8 ibft
kustomize build ibftk8/ibft | kubectl apply -f -
```


## rrr
```sh
bbake new -p 5 -k rrrk5 rrr
kustomize build rrrk5/rrr | kubectl apply -f -
```

See this projects loadtest [workflow](.github/workflows/loadtest-on-gcp.yaml)
for automation examples

# Debug a node

```sh
cd ~/workspace/raft5
```

Open the docker-compose.yaml and find the stanza for node0

```yaml
node0:
  <<: *node-defaults
  working_dir: /nodes/node0
  ports:
    - "8300:8300"
```

Make it look like this

```yaml
node0:
  <<: *node-debug
  working_dir: /nodes/node0
  ports:
    - "8300:8300"
    - "2345:2345"
```

Edit the .env file (if necessary) and ensure that QUORUM_SRC refers to the
location of your geth clone and DELVE_IMAGE has the name of an approprate delve
image. See the example [Dockerfile](./compose/delve-debug/Dockerfile-delve). If
you want to use the example, then

```
cd ~/workspace/benchblock/compose/delv-debug
docker build . -f Dockerfile-delve -t geth-delve:latest
```

And set DELVE_IMAGE=`geth-delve:latest` in the `.env` file.

In a vscode workspace which includes quorum create a debug launch
configuration. The resulting launch.json should look like this:

```json
[
    {
        "name": "geth node docker remote",
        "type": "go",
        "request": "attach",
        "mode": "remote",
        "port": 2345,
        "host": "127.0.0.1",
        "remotePath": "/go/src/quorum",
        "cwd": "~/workspace/quorum",
        // "trace": "log"
    }
]
```
See this
[guide](https://github.com/golang/vscode-go/blob/master/docs/debugging.md) for detailed help re debugging go with vscode.


Start node0 on its own `docker-compose up node0`

Once it starts listening start the debug target in vscode.

Note: the cwd needs to be the quorum clone directory in order for source level break points to work.  If you placed
the quorum clone according to the Setup section, the defaults should be ok.

# Setup

## docker

We provide a docker image as an alternative to installing the tools described below. 

`docker run -u $(id -u):$(id -g) -v $(pwd):$(pwd) -w $(pwd) robinbryce/bbake`

Is equivelant to installing all the host tools and running

`tusk -qf benchblock/tusk.yml`

## loadtool installation

```sh
ARCH=linux-amd64
TAG=v0.2.0
FILENAME=bbeth-${TAG}-${ARCH}.tar.gz

URL=https://github.com/robinbryce/benchblock/releases/download/${TAG}/${FILENAME}
curl -o ${FILENAME} -L $URL
tar -zxf ${FILENAME}
chmod a+x bbeth

# And put it on your PATH
```

## Host (linux)

Note: See the [Dockerfile](./Dockerfile) for full details

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

git clone https://github.com/robinbryce/benchblock.git
alias bbake='tusk -qf ~/workspace/benchblock/tusk.yml'
```

Please see [go-tusk](https://github.com/rliebz/tusk#readme), [yq](https://github.com/mikefarah/yq/blob/master/README.md) for up to installation details and information for other platforms.

The [go-quorum](https://github.com/ConsenSys/quorum.git) clone clone is needed to take advantage of  compose remote debug  configuration [^2], but otherwise can be ommitted

# Related work

## Chainhammer

The jupyter charting support owes much to the
[chainhammer](https://github.com/drandreaskrueger/chainhammer/blob/master/README.md)
project, which also supports a broader range of ledgers.

## Blockbench
The blockbench [project](https://github.com/ooibc88/blockbench) &
[paper](https://core.ac.uk/download/pdf/84912082.pdf) which take a deeper look
at the performance of different platforms in the face of different smart
contract use cases.

## ConsenSys dev quickstart

ConsenSys provid their own family of tools for configuring networks. The
developer quickstart can be found [here](https://github.com/ConsenSys/quorum-dev-quickstart)

[^1]: A pre-alpha implementation of this
  [paper](https://arxiv.org/pdf/1804.07391.pdf) for go-ethereum. Testing and
  development of which was a key motivation for this project.
