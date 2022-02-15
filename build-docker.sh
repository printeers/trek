#!/bin/bash
set -euxo pipefail

cd "$(dirname "$0")"

docker build -t ghcr.io/stack11/trek:latest .
