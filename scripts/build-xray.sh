#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
OUTPUT_DIR="${OUTPUT_DIR:-bin/xray}"
GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-amd64}"
GOMIPS="${GOMIPS:-softfloat}"
XRAY_VERSION="${XRAY_VERSION:-v26.7.28}"
XRAY_REPO_URL="${XRAY_REPO_URL:-https://github.com/XTLS/Xray-core.git}"
XRAY_SOURCE_DIR="${XRAY_SOURCE_DIR:-${ROOT_DIR}/.cache/xray-src/${XRAY_VERSION}}"

mkdir -p "${OUTPUT_DIR}" "$(dirname "${XRAY_SOURCE_DIR}")"
rm -f "${OUTPUT_DIR}/xray"

if [ ! -d "${XRAY_SOURCE_DIR}/.git" ]; then
	rm -rf "${XRAY_SOURCE_DIR}"
	git clone --depth 1 --branch "${XRAY_VERSION}" "${XRAY_REPO_URL}" "${XRAY_SOURCE_DIR}"
fi

echo "Building Xray ${XRAY_VERSION} for ${GOOS}/${GOARCH}"

if [ "${GOARCH}" = "mips" ] || [ "${GOARCH}" = "mipsle" ]; then
	echo "Using GOMIPS=${GOMIPS}"
	(
		cd "${XRAY_SOURCE_DIR}"
		GOTOOLCHAIN=go1.26.0 CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" GOMIPS="${GOMIPS}" \
			go build -trimpath -ldflags="-s -w" -o "${OUTPUT_DIR}/xray" ./main
	)
else
	(
		cd "${XRAY_SOURCE_DIR}"
		GOTOOLCHAIN=go1.26.0 CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" \
			go build -trimpath -ldflags="-s -w" -o "${OUTPUT_DIR}/xray" ./main
	)
fi

if command -v upx >/dev/null 2>&1; then
	echo "Compressing xray binary with UPX..."
	upx -9 "${OUTPUT_DIR}/xray"
else
	echo "UPX not found, skipping compression"
fi

echo "Built ${OUTPUT_DIR}/xray"
