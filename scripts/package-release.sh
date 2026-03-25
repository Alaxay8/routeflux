#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
VERSION="${VERSION:-}"

resolve_version() {
	if [ -n "${VERSION}" ]; then
		printf '%s\n' "${VERSION#v}"
		return
	fi

	if command -v git >/dev/null 2>&1; then
		version="$(git -C "${ROOT_DIR}" describe --tags --always --dirty 2>/dev/null || true)"
		if [ -n "${version}" ]; then
			printf '%s\n' "${version#v}"
			return
		fi
	fi

	printf '0.0.0-dev\n'
}

build_release_asset() {
	package_arch="$1"
	output_dir="$2"
	goarch="$3"
	gomips="${4:-}"

	if [ -n "${gomips}" ]; then
		OUTPUT_DIR="${output_dir}" GOARCH="${goarch}" GOMIPS="${gomips}" \
			"${ROOT_DIR}/scripts/build-openwrt.sh"
	else
		OUTPUT_DIR="${output_dir}" GOARCH="${goarch}" \
			"${ROOT_DIR}/scripts/build-openwrt.sh"
	fi

	VERSION="${RELEASE_VERSION}" ARCH="${package_arch}" BINARY_PATH="${output_dir}/routeflux" \
		"${ROOT_DIR}/scripts/package-openwrt.sh"
}

RELEASE_VERSION="$(resolve_version)"

build_release_asset "mipsel_24kc" "${ROOT_DIR}/bin/openwrt/mipsel_24kc" "mipsle" "softfloat"
build_release_asset "x86_64" "${ROOT_DIR}/bin/openwrt/x86_64" "amd64"
"${ROOT_DIR}/scripts/render-install.sh" "${RELEASE_VERSION}" "${ROOT_DIR}/dist/install.sh" "mipsel_24kc" "x86_64"
