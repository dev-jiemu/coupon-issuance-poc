#!/bin/bash
set -e

export PATH="$PATH:$(go env GOPATH)/bin"

PROTO_DIR=proto
OUT_DIR=pkg/gen

mkdir -p $OUT_DIR

protoc -I=$PROTO_DIR $PROTO_DIR/v1/campaign.proto $PROTO_DIR/v1/coupon.proto --go_out=paths=source_relative:$OUT_DIR --go-grpc_out=paths=source_relative:$OUT_DIR

echo "Generated files:"
find $OUT_DIR -type f | sort