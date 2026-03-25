#!/bin/sh
set -eu

OUTPUT_DIR="${OUTPUT_DIR:-bin/openwrt}"
GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-mipsle}"
GOMIPS="${GOMIPS:-softfloat}"

mkdir -p "${OUTPUT_DIR}"

echo "Building RouteFlux for ${GOOS}/${GOARCH}"

if [ "${GOARCH}" = "mips" ] || [ "${GOARCH}" = "mipsle" ]; then
	echo "Using GOMIPS=${GOMIPS}"
	CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" GOMIPS="${GOMIPS}" \
		go build -trimpath -ldflags="-s -w" -o "${OUTPUT_DIR}/routeflux" ./cmd/routeflux
else
	CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" \
		go build -trimpath -ldflags="-s -w" -o "${OUTPUT_DIR}/routeflux" ./cmd/routeflux
fi

echo "Built ${OUTPUT_DIR}/routeflux"
