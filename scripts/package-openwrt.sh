#!/bin/sh
set -eu

PKG_DIR="${PKG_DIR:-dist/routeflux-ipk}"
VERSION="${VERSION:-0.1.0}"
ARCH="${ARCH:-all}"

rm -rf "${PKG_DIR}"
mkdir -p "${PKG_DIR}/usr/bin" "${PKG_DIR}/CONTROL"

cp bin/openwrt/routeflux "${PKG_DIR}/usr/bin/routeflux"

cat > "${PKG_DIR}/CONTROL/control" <<EOF
Package: routeflux
Version: ${VERSION}
Architecture: ${ARCH}
Maintainer: Alexey
Description: RouteFlux OpenWrt subscription proxy manager
EOF

tar -C "${PKG_DIR}" -czf "dist/routeflux_${VERSION}_${ARCH}.tar.gz" .
echo "Created dist/routeflux_${VERSION}_${ARCH}.tar.gz"
