#!/bin/sh
set -eu

OUTPUT_DIR="${OUTPUT_DIR:-bin/openwrt}"
GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-mipsle}"
GOMIPS="${GOMIPS:-softfloat}"
VERSION="${VERSION:-}"

resolve_version() {
	if [ -n "${VERSION}" ]; then
		printf '%s\n' "${VERSION#v}"
		return
	fi

	if command -v git >/dev/null 2>&1; then
		version="$(git describe --tags --always --dirty 2>/dev/null || true)"
		if [ -n "${version}" ]; then
			printf '%s\n' "${version#v}"
			return
		fi
	fi

	printf '0.0.0-dev\n'
}

resolve_commit() {
	if command -v git >/dev/null 2>&1; then
		commit="$(git rev-parse --short HEAD 2>/dev/null || true)"
		if [ -n "${commit}" ]; then
			printf '%s\n' "${commit}"
			return
		fi
	fi

	printf 'unknown\n'
}

resolve_build_date() {
	if [ -n "${SOURCE_DATE_EPOCH:-}" ]; then
		if date -u -d "@${SOURCE_DATE_EPOCH}" '+%Y-%m-%dT%H:%M:%SZ' >/dev/null 2>&1; then
			date -u -d "@${SOURCE_DATE_EPOCH}" '+%Y-%m-%dT%H:%M:%SZ'
			return
		fi
		date -u -r "${SOURCE_DATE_EPOCH}" '+%Y-%m-%dT%H:%M:%SZ'
		return
	fi

	date -u '+%Y-%m-%dT%H:%M:%SZ'
}

VERSION="$(resolve_version)"
COMMIT="$(resolve_commit)"
BUILD_DATE="$(resolve_build_date)"
LDFLAGS="-s -w -X github.com/Alaxay8/routeflux/internal/buildinfo.Version=${VERSION} -X github.com/Alaxay8/routeflux/internal/buildinfo.Commit=${COMMIT} -X github.com/Alaxay8/routeflux/internal/buildinfo.BuildDate=${BUILD_DATE}"

mkdir -p "${OUTPUT_DIR}"

echo "Building RouteFlux for ${GOOS}/${GOARCH}"

if [ "${GOARCH}" = "mips" ] || [ "${GOARCH}" = "mipsle" ]; then
	echo "Using GOMIPS=${GOMIPS}"
	CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" GOMIPS="${GOMIPS}" \
		go build -trimpath -ldflags="${LDFLAGS}" -o "${OUTPUT_DIR}/routeflux" ./cmd/routeflux
else
	CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" \
		go build -trimpath -ldflags="${LDFLAGS}" -o "${OUTPUT_DIR}/routeflux" ./cmd/routeflux
fi

echo "Built ${OUTPUT_DIR}/routeflux"
