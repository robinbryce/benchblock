ARG QUORUM_TAG=main
ARG QUORUM_RRR_TAG=main
ARG RRRCTL_TAG=main

FROM golang:1.15-buster as go-builder

ENV GOBIN=/go/bin

WORKDIR go/bbeth
COPY go/bbeth/go.mod go/bbeth/go.sum ./
RUN go mod download

COPY go/bbeth/cmd cmd
COPY go/bbeth/client client
COPY go/bbeth/collect collect
COPY go/bbeth/load load
COPY go/bbeth/root root

COPY go/bbeth/*.go .
RUN find . && go build -o ${GOBIN}/bbeth main.go

# FROM quorumengineering/quorum:${QUORUM_TAG} as quorum (its an alpine image)
FROM robustroundrobin/rrrctl:${RRRCTL_TAG} as rrrctl
FROM robustroundrobin/geth:${QUORUM_RRR_TAG} as quorum_rrr

FROM python:3.9-slim-bullseye as py-builder

ENV PATH /usr/local/bin:${PATH}
ENV BBAKE_GETH_BIN=/usr/local/bin/geth
ENV BBAKE_GETH_RRR_BIN=/usr/local/bin/geth-rrr
ENV BBAKE_RRRCTL_BIN=/usr/local/bin/rrrctl

RUN DEBIAN_FRONTEND=noninteractive apt-get update \
  && apt-get upgrade -y --no-install-recommends \
  && apt-get install -y \
        autoconf \
        automake \
        pkg-config \
        build-essential \
        libffi-dev \
        libtool \
        gettext-base

WORKDIR /bbake

COPY requirements.txt requirements.txt
COPY jupyter-support/requirements.txt jupyter-support/requirements.txt

# TODO --user install and COPY /root/.local into clean python-slim
RUN \
    pip install --user dnspython \
    && pip install --user -r requirements.txt \
    && pip install --user -r jupyter-support/requirements.txt \
    && rm -rf /tmp/pip

FROM python:3.9-slim-bullseye

ENV YQ_BINARY=yq_linux_amd64
ENV YQ_VERSION=v4.12.0
ENV TUSK_VERSION latest
ENV PATH /usr/local/bin:${PATH}
ENV BBAKE_GETH_BIN=/usr/local/bin/geth
ENV BBAKE_GETH_RRR_BIN=/usr/local/bin/geth-rrr
ENV BBAKE_RRRCTL_BIN=/usr/local/bin/rrrctl

RUN DEBIAN_FRONTEND=noninteractive apt-get update \
  && apt-get upgrade -y --no-install-recommends \
  && apt-get install -y \
        gcc \
        curl \
        jq \
        ca-certificates \
  && apt-get clean \
  && curl -sL https://git.io/tusk | bash -s -- -b /usr/local/bin ${TUSK_VERSION} \
  && curl -sL https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/${YQ_BINARY}.tar.gz \
     | tar xz && mv ${YQ_BINARY} /usr/local/bin/yq


WORKDIR /bbake

COPY --from=go-builder /go/bin/bbeth /usr/local/bin/bbeth
COPY --from=quorum_rrr /usr/local/bin/geth /usr/local/bin/geth
COPY --from=quorum_rrr /usr/local/bin/geth /usr/local/bin/geth-rrr
COPY --from=rrrctl /usr/local/bin/rrrctl /usr/local/bin/rrrctl

COPY --from=py-builder /root/.local /root/.local
COPY requirements.txt requirements.txt
COPY jupyter-support jupyter-support

COPY compose compose
COPY configs configs
COPY k8s k8s
COPY tuskfiles tuskfiles
COPY tusk.yml tusk.yml
COPY entrypoint.sh entrypoint.sh

ENTRYPOINT [ "/bbake/entrypoint.sh" ]
