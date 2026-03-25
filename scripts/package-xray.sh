#!/bin/sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
PKG_DIR="${PKG_DIR:-dist/xray-runtime}"
ARCH="${ARCH:-mipsel_24kc}"
BINARY_PATH="${BINARY_PATH:-${ROOT_DIR}/bin/xray/xray}"
SERVICE_PATH="${SERVICE_PATH:-${ROOT_DIR}/openwrt/root/etc/init.d/xray}"

resolve_version() {
	if [ -n "${VERSION:-}" ]; then
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

create_tarball() {
	src_dir="$1"
	out_file="$2"
	out_file_dir="$(CDPATH= cd -- "$(dirname "${out_file}")" && pwd)"
	out_file="${out_file_dir}/$(basename "${out_file}")"

	(
		cd "${src_dir}"
		entries="$(find . -mindepth 1 -maxdepth 1 -print | LC_ALL=C sort)"
		if command -v bsdtar >/dev/null 2>&1; then
			# shellcheck disable=SC2086
			COPYFILE_DISABLE=1 bsdtar --format ustar --uid 0 --gid 0 --uname root --gname root -czf "${out_file}" ${entries}
			return
		fi

		if command -v tar >/dev/null 2>&1; then
			# shellcheck disable=SC2086
			COPYFILE_DISABLE=1 tar --format=ustar --owner=0 --group=0 --numeric-owner -czf "${out_file}" ${entries}
			return
		fi

		printf 'neither bsdtar nor tar is available\n' >&2
		exit 1
	)
}

VERSION="$(resolve_version)"
DATA_DIR="${PKG_DIR}/data"
TARBALL_PATH="${ROOT_DIR}/dist/xray_${VERSION}_${ARCH}.tar.gz"

rm -rf "${PKG_DIR}"
mkdir -p "${ROOT_DIR}/dist" "${DATA_DIR}/usr/bin" "${DATA_DIR}/etc/init.d"

cp "${BINARY_PATH}" "${DATA_DIR}/usr/bin/xray"
cp "${SERVICE_PATH}" "${DATA_DIR}/etc/init.d/xray"
chmod 0755 "${DATA_DIR}/usr/bin/xray" "${DATA_DIR}/etc/init.d/xray"

create_tarball "${DATA_DIR}" "${TARBALL_PATH}"
echo "Created ${TARBALL_PATH}"
