#!/bin/bash
set -e
docker build -t ghcr.io/stack11/trek/migra:latest - < Dockerfile.migra
docker build -t ghcr.io/stack11/trek:latest .
