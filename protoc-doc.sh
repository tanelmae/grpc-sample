#!/usr/bin/env bash
set -e

cd "$( dirname "${BASH_SOURCE[0]}" )"

if ! command -v protoc &> /dev/null; then
    echo "protoc not found on PATH"
    echo "See https://grpc.io/docs/protoc-installation/"
    exit 1
fi

if ! command -v protoc-gen-doc &> /dev/null; then
    echo "protoc-gen-doc not found on PATH"
    echo "Will install it to ${GOPATH}/bin"
    go get -u github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc
fi

protoc --doc_out=./pb --doc_opt=html,index.html pb/service.proto
