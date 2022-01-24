#!/bin/bash

set -euxo pipefail
docker build -t ghcr.io/stack11/trek:latest .
