#!/bin/bash
set -euxo pipefail

# This script builds migra in a Dockerfile-based image build, and then copies
# the migra binary from the docker image to the directory where this script is
# located.

cd "$(dirname "$0")"

docker build -t build-migra:latest - < build-migra.Dockerfile
id="$(docker run --rm -d build-migra:latest sleep infinity)"
function cleanup() {
    docker kill "$id" > /dev/null 2>&1 || true
}
trap cleanup EXIT
docker cp "$id":/app/out/migra ./migra
cleanup
