#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
PROTO_DIR="$SCRIPT_DIR/proto"
GEN_DIR="$PROJECT_ROOT/pkg/gen"

rm -rf "$GEN_DIR/v1"
mkdir -p "$GEN_DIR"

# Check if buf is installed
if ! command -v buf &> /dev/null; then
    echo "buf is not installed. Installing buf..."
    # Install buf (you may need to adjust this based on the OS)
    go install github.com/bufbuild/buf/cmd/buf@latest
fi

cd "$PROTO_DIR"
buf generate

echo "Generated files:"
find "$GEN_DIR" -type f | sort
