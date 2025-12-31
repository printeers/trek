#!/bin/bash
set -euxo pipefail

rm -rf example
mkdir example
cd example

TREK_VERSION=latest \
TREK_MODEL_NAME=foo \
TREK_DATABASE_NAME=bar \
TREK_ROLES=alice,bob \
go run .. init

go run .. check

go run .. generate --stdout
