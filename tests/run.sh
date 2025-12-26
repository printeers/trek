#!/bin/bash
set -euxo pipefail
cd "$(dirname "$0")"

rm -rf output
mkdir -p output

(
  cd output

  function trek {
    go run ../.. "${@}"
  }

  TREK_VERSION=latest \
  TREK_MODEL_NAME=santas_warehouse \
  TREK_DATABASE_NAME=north_pole \
  TREK_ROLES=santa,worker \
  trek init

  trek check

  for file in ../stages/*.dbm; do
    cp "$file" santas_warehouse.dbm

    stage_name=$(basename "$file" | cut -d "-" -f 2 | cut -d "." -f 1)

    trek generate "$stage_name"

    trek check
  done
)
