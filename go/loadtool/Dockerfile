FROM golang:1.15.14-alpine as builder
RUN apk add --no-cache make gcc musl-dev linux-headers git

ENV GOBIN=/go/bin

WORKDIR /build
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY *.go .
ADD cmd cmd
ADD loader loader

RUN go install github.com/robinbryce/blockbench/loadtool
RUN find /build

FROM alpine:3.13.0

RUN apk add --no-cache ca-certificates

COPY --from=builder /go/bin/loadtool /usr/local/bin/

ENTRYPOINT ["loadtool"]
