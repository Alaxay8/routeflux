#!/bin/sh
set -eu

if [ "$#" -lt 3 ]; then
	printf '%s\n' "usage: render-install.sh <version> <output-path> <arch> [arch...]" >&2
	exit 1
fi

ROOT_DIR="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
VERSION="${1#v}"
OUTPUT_PATH="$2"
shift 2

SUPPORTED_ARCHES="$*"
TEMPLATE_PATH="${ROOT_DIR}/scripts/install.sh.tmpl"

mkdir -p "$(dirname "${OUTPUT_PATH}")"

sed \
	-e "s|__ROUTEFLUX_VERSION__|${VERSION}|g" \
	-e "s|__SUPPORTED_ARCHES__|${SUPPORTED_ARCHES}|g" \
	"${TEMPLATE_PATH}" > "${OUTPUT_PATH}"

chmod 0755 "${OUTPUT_PATH}"
printf 'Created %s\n' "${OUTPUT_PATH}"
