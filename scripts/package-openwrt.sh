#!/bin/sh
set -eu

PKG_DIR="${PKG_DIR:-dist/routeflux-ipk}"
VERSION="${VERSION:-0.1.0}"
ARCH="${ARCH:-all}"
ROOT_DIR="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"

rm -rf "${PKG_DIR}"
mkdir -p \
	"${PKG_DIR}/usr/bin" \
	"${PKG_DIR}/usr/share/luci/menu.d" \
	"${PKG_DIR}/usr/share/rpcd/acl.d" \
	"${PKG_DIR}/www/luci-static/resources/view/routeflux" \
	"${PKG_DIR}/CONTROL"

cp "${ROOT_DIR}/bin/openwrt/routeflux" "${PKG_DIR}/usr/bin/routeflux"
cp "${ROOT_DIR}/luci-app-routeflux/root/usr/share/luci/menu.d/luci-app-routeflux.json" \
	"${PKG_DIR}/usr/share/luci/menu.d/luci-app-routeflux.json"
cp "${ROOT_DIR}/luci-app-routeflux/root/usr/share/rpcd/acl.d/luci-app-routeflux.json" \
	"${PKG_DIR}/usr/share/rpcd/acl.d/luci-app-routeflux.json"
cp "${ROOT_DIR}/luci-app-routeflux/htdocs/luci-static/resources/view/routeflux/"*.js \
	"${PKG_DIR}/www/luci-static/resources/view/routeflux/"

cat > "${PKG_DIR}/CONTROL/control" <<EOF
Package: routeflux
Version: ${VERSION}
Architecture: ${ARCH}
Maintainer: Alexey
Description: RouteFlux OpenWrt subscription proxy manager with LuCI frontend files
EOF

cat > "${PKG_DIR}/CONTROL/postinst" <<'EOF'
#!/bin/sh
set -eu

if [ -z "${IPKG_INSTROOT:-}" ]; then
	rm -f /tmp/luci-indexcache
	rm -rf /tmp/luci-modulecache
	/etc/init.d/rpcd reload >/dev/null 2>&1 || true
	/etc/init.d/uhttpd reload >/dev/null 2>&1 || true
fi
EOF
chmod 0755 "${PKG_DIR}/CONTROL/postinst"

tar -C "${PKG_DIR}" -czf "dist/routeflux_${VERSION}_${ARCH}.tar.gz" .
echo "Created dist/routeflux_${VERSION}_${ARCH}.tar.gz"
