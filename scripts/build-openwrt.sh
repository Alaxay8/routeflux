#!/bin/sh
set -eu

OUTPUT_DIR="${OUTPUT_DIR:-bin/openwrt}"
GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-mipsle}"
GOMIPS="${GOMIPS:-softfloat}"

mkdir -p "${OUTPUT_DIR}"

echo "Building RouteFlux for ${GOOS}/${GOARCH} (${GOMIPS})"
CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" GOMIPS="${GOMIPS}" \
	go build -trimpath -ldflags="-s -w" -o "${OUTPUT_DIR}/routeflux" ./cmd/routeflux

echo "Built ${OUTPUT_DIR}/routeflux"
