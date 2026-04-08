#!/bin/bash
# build.sh - Build Docker image for ChronosDB

set -e

VERSION=${1:-latest}
REGISTRY=${REGISTRY:-chronosdb}

echo "Building ChronosDB Docker image..."
docker build -f deployments/docker/Dockerfile -t ${REGISTRY}/chronosdb:${VERSION} .

echo "Image built: ${REGISTRY}/chronosdb:${VERSION}"
