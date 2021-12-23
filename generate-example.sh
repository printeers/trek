#!/bin/bash
set -euxo pipefail

rm -rf example
mkdir example
cd example

TREK_VERSION=latest \
MODEL_NAME=foo \
DATABASE_NAME=bar \
DATABASE_USERS=alice,bob \
go run .. init
