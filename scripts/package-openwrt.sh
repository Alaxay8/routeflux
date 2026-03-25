#!/bin/sh
set -eu

PKG_DIR="${PKG_DIR:-dist/routeflux-ipk}"
ARCH="${ARCH:-mipsel_24kc}"
ROOT_DIR="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
BINARY_PATH="${BINARY_PATH:-${ROOT_DIR}/bin/openwrt/routeflux}"
DATA_DIR="${PKG_DIR}/data"
CONTROL_DIR="${PKG_DIR}/control"
WORK_DIR="${PKG_DIR}/work"
PACKAGE_NAME="${PACKAGE_NAME:-routeflux}"

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

VERSION="$(resolve_version)"
IPK_PATH="${ROOT_DIR}/dist/${PACKAGE_NAME}_${VERSION}_${ARCH}.ipk"
TARBALL_PATH="${ROOT_DIR}/dist/${PACKAGE_NAME}_${VERSION}_${ARCH}.tar.gz"

rm -rf "${PKG_DIR}"
mkdir -p "${ROOT_DIR}/dist"
mkdir -p \
	"${DATA_DIR}/usr/bin" \
	"${DATA_DIR}/etc/init.d" \
	"${DATA_DIR}/usr/share/luci/menu.d" \
	"${DATA_DIR}/usr/share/rpcd/acl.d" \
	"${DATA_DIR}/www/luci-static/resources/view/routeflux" \
	"${CONTROL_DIR}" \
	"${WORK_DIR}"

cp "${BINARY_PATH}" "${DATA_DIR}/usr/bin/routeflux"
cp "${ROOT_DIR}/openwrt/root/etc/init.d/routeflux" "${DATA_DIR}/etc/init.d/routeflux"
chmod 0755 "${DATA_DIR}/etc/init.d/routeflux"
cp "${ROOT_DIR}/luci-app-routeflux/root/usr/share/luci/menu.d/luci-app-routeflux.json" \
	"${DATA_DIR}/usr/share/luci/menu.d/luci-app-routeflux.json"
cp "${ROOT_DIR}/luci-app-routeflux/root/usr/share/rpcd/acl.d/luci-app-routeflux.json" \
	"${DATA_DIR}/usr/share/rpcd/acl.d/luci-app-routeflux.json"
cp "${ROOT_DIR}/luci-app-routeflux/htdocs/luci-static/resources/view/routeflux/"*.js \
	"${DATA_DIR}/www/luci-static/resources/view/routeflux/"

cat > "${CONTROL_DIR}/control" <<EOF
Package: ${PACKAGE_NAME}
Version: ${VERSION}
Architecture: ${ARCH}
Maintainer: Alexey
License: MIT
Section: net
Priority: optional
Description: RouteFlux OpenWrt subscription proxy manager with LuCI frontend files
EOF

cat > "${CONTROL_DIR}/postinst" <<'EOF'
#!/bin/sh
set -eu

if [ -z "${IPKG_INSTROOT:-}" ]; then
	chmod 0755 /etc/init.d/routeflux >/dev/null 2>&1 || true
	rm -f /tmp/luci-indexcache
	rm -rf /tmp/luci-modulecache
	/etc/init.d/rpcd reload >/dev/null 2>&1 || true
	/etc/init.d/uhttpd reload >/dev/null 2>&1 || true
fi
EOF
chmod 0755 "${CONTROL_DIR}/postinst"

printf '2.0\n' > "${WORK_DIR}/debian-binary"

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

create_tarball "${CONTROL_DIR}" "${WORK_DIR}/control.tar.gz"
create_tarball "${DATA_DIR}" "${WORK_DIR}/data.tar.gz"
create_tarball "${DATA_DIR}" "${TARBALL_PATH}"

rm -f "${IPK_PATH}"
printf '!<arch>\n' > "${IPK_PATH}"

write_ar_member() {
	name="$1"
	file="$2"
	size="$(wc -c < "${file}" | tr -d ' ')"
	timestamp="$(date +%s)"

	printf '%-16s%-12s%-6s%-6s%-8s%-10s`\n' \
		"${name}/" \
		"${timestamp}" \
		"0" \
		"0" \
		"100644" \
		"${size}" >> "${IPK_PATH}"
	cat "${file}" >> "${IPK_PATH}"

	if [ $((size % 2)) -ne 0 ]; then
		printf '\n' >> "${IPK_PATH}"
	fi
}

write_ar_member "debian-binary" "${WORK_DIR}/debian-binary"
write_ar_member "control.tar.gz" "${WORK_DIR}/control.tar.gz"
write_ar_member "data.tar.gz" "${WORK_DIR}/data.tar.gz"

echo "Created ${IPK_PATH}"
echo "Created ${TARBALL_PATH}"
