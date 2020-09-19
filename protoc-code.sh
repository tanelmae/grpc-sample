#!/usr/bin/env bash
set -e

cd "$( dirname "${BASH_SOURCE[0]}" )"

if ! command -v protoc &> /dev/null; then
    echo "protoc not found on PATH"
    echo "See https://grpc.io/docs/protoc-installation/"
    exit 1
fi

if ! command -v protoc-gen-go &> /dev/null; then
    echo "protoc-gen-go not found on PATH"
    echo "Will install it to ${GOPATH}/bin"
    go install github.com/golang/protobuf/protoc-gen-go
fi

if ! command -v protoc-go-inject-tag &> /dev/null; then
    echo "protoc-go-inject-tag not found on PATH"
    echo "Will install it to ${GOPATH}/bin"
    go install github.com/favadi/protoc-go-inject-tag
fi

protoc --go_out=plugins=grpc:. --go_opt=paths=source_relative pb/service.proto
protoc-go-inject-tag -input=./pb/service.pb.go
