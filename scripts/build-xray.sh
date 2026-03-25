#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
OUTPUT_DIR="${OUTPUT_DIR:-bin/xray}"
GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-amd64}"
GOMIPS="${GOMIPS:-softfloat}"
XRAY_VERSION="${XRAY_VERSION:-v26.2.6}"
XRAY_REPO_URL="${XRAY_REPO_URL:-https://github.com/XTLS/Xray-core.git}"
XRAY_SOURCE_DIR="${XRAY_SOURCE_DIR:-${ROOT_DIR}/.cache/xray-src/${XRAY_VERSION}}"

mkdir -p "${OUTPUT_DIR}" "$(dirname "${XRAY_SOURCE_DIR}")"

if [ ! -d "${XRAY_SOURCE_DIR}/.git" ]; then
	rm -rf "${XRAY_SOURCE_DIR}"
	git clone --depth 1 --branch "${XRAY_VERSION}" "${XRAY_REPO_URL}" "${XRAY_SOURCE_DIR}"
fi

echo "Building Xray ${XRAY_VERSION} for ${GOOS}/${GOARCH}"

if [ "${GOARCH}" = "mips" ] || [ "${GOARCH}" = "mipsle" ]; then
	echo "Using GOMIPS=${GOMIPS}"
	(
		cd "${XRAY_SOURCE_DIR}"
		GOTOOLCHAIN=auto CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" GOMIPS="${GOMIPS}" \
			go build -trimpath -ldflags="-s -w" -o "${OUTPUT_DIR}/xray" ./main
	)
else
	(
		cd "${XRAY_SOURCE_DIR}"
		GOTOOLCHAIN=auto CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" \
			go build -trimpath -ldflags="-s -w" -o "${OUTPUT_DIR}/xray" ./main
	)
fi

echo "Built ${OUTPUT_DIR}/xray"
