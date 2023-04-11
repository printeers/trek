#!/bin/bash
set -euxo pipefail

cd "$(dirname "$0")/.."

docker build -t ghcr.io/printeers/trek/migra:latest - < migra/Dockerfile
id="$(docker run --rm -d ghcr.io/printeers/trek/migra:latest sleep infinity)"
function cleanup() {
    docker kill "$id" > /dev/null 2>&1 || true
}
trap cleanup EXIT
docker cp "$id":/app/out/migra internal/embed/bin/migra
cleanup
