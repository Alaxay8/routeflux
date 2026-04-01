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

build_xray_asset() {
	package_arch="$1"
	output_dir="$2"
	goarch="$3"
	gomips="${4:-}"

	if [ -n "${gomips}" ]; then
		OUTPUT_DIR="${output_dir}" GOARCH="${goarch}" GOMIPS="${gomips}" \
			"${ROOT_DIR}/scripts/build-xray.sh"
	else
		OUTPUT_DIR="${output_dir}" GOARCH="${goarch}" \
			"${ROOT_DIR}/scripts/build-xray.sh"
	fi

	VERSION="${RELEASE_VERSION}" ARCH="${package_arch}" BINARY_PATH="${output_dir}/xray" \
		"${ROOT_DIR}/scripts/package-xray.sh"
}

RELEASE_VERSION="$(resolve_version)"

build_release_asset "mipsel_24kc" "${ROOT_DIR}/bin/openwrt/mipsel_24kc" "mipsle" "softfloat"
build_release_asset "x86_64" "${ROOT_DIR}/bin/openwrt/x86_64" "amd64"
build_release_asset "aarch64_cortex-a53" "${ROOT_DIR}/bin/openwrt/aarch64_cortex-a53" "arm64"
build_xray_asset "mipsel_24kc" "${ROOT_DIR}/bin/xray/mipsel_24kc" "mipsle" "softfloat"
build_xray_asset "x86_64" "${ROOT_DIR}/bin/xray/x86_64" "amd64"
build_xray_asset "aarch64_cortex-a53" "${ROOT_DIR}/bin/xray/aarch64_cortex-a53" "arm64"
"${ROOT_DIR}/scripts/render-install.sh" "${RELEASE_VERSION}" "${ROOT_DIR}/dist/install.sh" "mipsel_24kc" "x86_64" "aarch64_cortex-a53"
cp "${ROOT_DIR}/scripts/uninstall.sh" "${ROOT_DIR}/dist/uninstall.sh"
chmod 0755 "${ROOT_DIR}/dist/uninstall.sh"
