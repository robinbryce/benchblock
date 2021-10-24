ARG QUORUM_TAG=main
ARG QUORUM_RRR_TAG=main
ARG RRRCTL_TAG=main

FROM golang:1.15-buster as go-builder

ENV GOBIN=/go/bin

WORKDIR go/bbencheth
COPY go/bbencheth/go.mod go/bbencheth/go.sum ./
RUN go mod download

COPY go/bbencheth/cmd cmd
COPY go/bbencheth/client client
COPY go/bbencheth/collect collect
COPY go/bbencheth/load load
COPY go/bbencheth/root root

COPY go/bbencheth/*.go .
RUN find . && go build -o ${GOBIN}/bbencheth main.go

# FROM quorumengineering/quorum:${QUORUM_TAG} as quorum (its an alpine image)
FROM robustroundrobin/rrrctl:${RRRCTL_TAG} as rrrctl
FROM robustroundrobin/geth:${QUORUM_RRR_TAG} as quorum_rrr

FROM python:3.9-slim-bullseye as py-builder

ENV PATH /usr/local/bin:${PATH}
ENV BBENCH_GETH_BIN=/usr/local/bin/geth
ENV BBENCH_GETH_RRR_BIN=/usr/local/bin/geth-rrr
ENV BBENCH_RRRCTL_BIN=/usr/local/bin/rrrctl

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

WORKDIR /bbench

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
ENV BBENCH_GETH_BIN=/usr/local/bin/geth
ENV BBENCH_GETH_RRR_BIN=/usr/local/bin/geth-rrr
ENV BBENCH_RRRCTL_BIN=/usr/local/bin/rrrctl

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


WORKDIR /bbench

COPY --from=go-builder /go/bin/bbencheth /usr/local/bin/bbencheth
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

ENTRYPOINT [ "/bbench/entrypoint.sh" ]
