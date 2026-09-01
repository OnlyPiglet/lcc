#!/bin/sh
set -eu

output=${1:-dist/every}
output_dir=$(dirname "$output")
mkdir -p "$output_dir"
go build -trimpath -o "$output" ./cmd/every
printf 'built %s\n' "$output"
