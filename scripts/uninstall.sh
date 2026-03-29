#!/bin/sh
set -eu

ROUTEFLUX_INSTALL_ROOT="${ROUTEFLUX_INSTALL_ROOT:-/}"
ROUTEFLUX_ROOT_OVERRIDE="${ROUTEFLUX_ROOT:-}"
ROUTEFLUX_XRAY_BINARY_PATH="${ROUTEFLUX_XRAY_BINARY:-}"
ROUTEFLUX_XRAY_SERVICE_PATH="${ROUTEFLUX_XRAY_SERVICE:-}"
ROUTEFLUX_XRAY_CONFIG_PATH="${ROUTEFLUX_XRAY_CONFIG:-}"
ROUTEFLUX_DRY_RUN=0

usage() {
	printf '%s\n' "RouteFlux OpenWrt uninstaller"
	printf '%s\n' ""
	printf '%s\n' "Usage: uninstall.sh [--install-root <path>] [--dry-run]"
}

log() {
	printf '%s\n' "$*"
}

die() {
	printf 'error: %s\n' "$*" >&2
	exit 1
}

scope_path() {
	path="$1"

	case "${path}" in
		/*)
			if [ "${ROUTEFLUX_INSTALL_ROOT}" = "/" ]; then
				printf '%s\n' "${path}"
			else
				printf '%s%s\n' "${ROUTEFLUX_INSTALL_ROOT%/}" "${path}"
			fi
			;;
		*)
			if [ "${ROUTEFLUX_INSTALL_ROOT}" = "/" ]; then
				printf '%s\n' "${path}"
			else
				printf '%s/%s\n' "${ROUTEFLUX_INSTALL_ROOT%/}" "${path}"
			fi
			;;
	esac
}

routeflux_root_path() {
	path="${ROUTEFLUX_ROOT_OVERRIDE:-/etc/routeflux}"
	scope_path "${path}"
}

routeflux_binary_path() {
	scope_path "/usr/bin/routeflux"
}

routeflux_service_path() {
	scope_path "/etc/init.d/routeflux"
}

xray_binary_path() {
	path="${ROUTEFLUX_XRAY_BINARY_PATH:-/usr/bin/xray}"
	scope_path "${path}"
}

xray_service_path() {
	path="${ROUTEFLUX_XRAY_SERVICE_PATH:-/etc/init.d/xray}"
	scope_path "${path}"
}

xray_config_path() {
	path="${ROUTEFLUX_XRAY_CONFIG_PATH:-/etc/xray/config.json}"
	scope_path "${path}"
}

routeflux_cron_helper_path() {
	scope_path "/usr/libexec/routeflux-cron"
}

require_root_if_needed() {
	if [ "${ROUTEFLUX_INSTALL_ROOT}" = "/" ] && [ "$(id -u)" -ne 0 ]; then
		die "run this uninstaller as root on the router, or use --install-root for a staging directory"
	fi
}

run_service_if_present() {
	script="$1"
	action="$2"

	[ -x "${script}" ] || return 0
	"${script}" "${action}" >/dev/null 2>&1 || return 1
}

remove_path() {
	path="$1"

	if [ -e "${path}" ] || [ -L "${path}" ]; then
		log "Removing ${path}"
		rm -rf "${path}"
	fi
}

remove_glob() {
	pattern="$1"

	for path in ${pattern}; do
		if [ -e "${path}" ] || [ -L "${path}" ]; then
			log "Removing ${path}"
			rm -rf "${path}"
		fi
	done
}

while [ "$#" -gt 0 ]; do
	case "$1" in
		--install-root)
			[ "$#" -ge 2 ] || die "missing value for --install-root"
			ROUTEFLUX_INSTALL_ROOT="$2"
			shift 2
			;;
		--dry-run)
			ROUTEFLUX_DRY_RUN=1
			shift
			;;
		-h|--help)
			usage
			exit 0
			;;
		*)
			die "unknown argument: $1"
			;;
	esac
done

require_root_if_needed

routeflux_root="$(routeflux_root_path)"
routeflux_binary="$(routeflux_binary_path)"
routeflux_service="$(routeflux_service_path)"
xray_binary="$(xray_binary_path)"
xray_service="$(xray_service_path)"
xray_config="$(xray_config_path)"
xray_config_dir="$(dirname "${xray_config}")"
routeflux_cron_helper="$(routeflux_cron_helper_path)"
rpcd_service="$(scope_path "/etc/init.d/rpcd")"
uhttpd_service="$(scope_path "/etc/init.d/uhttpd")"

if [ "${ROUTEFLUX_DRY_RUN}" = "1" ]; then
	log "RouteFlux root: ${routeflux_root}"
	log "RouteFlux binary: ${routeflux_binary}"
	log "RouteFlux service: ${routeflux_service}"
	log "Xray binary: ${xray_binary}"
	log "Xray service: ${xray_service}"
	log "Xray config: ${xray_config}"
	exit 0
fi

if [ -x "${routeflux_binary}" ]; then
	"${routeflux_binary}" --root "${routeflux_root}" disconnect >/dev/null 2>&1 || true
	"${routeflux_binary}" --root "${routeflux_root}" firewall disable >/dev/null 2>&1 || true
fi

run_service_if_present "${routeflux_service}" stop || true
run_service_if_present "${routeflux_service}" disable || true
run_service_if_present "${xray_service}" stop || true
run_service_if_present "${xray_service}" disable || true
ROUTEFLUX_INSTALL_ROOT="${ROUTEFLUX_INSTALL_ROOT}" "${routeflux_cron_helper}" remove-xray-log-retention >/dev/null 2>&1 || true

remove_path "${routeflux_binary}"
remove_path "${routeflux_service}"
remove_path "${routeflux_root}"

remove_path "$(scope_path "/usr/share/luci/menu.d/luci-app-routeflux.json")"
remove_path "$(scope_path "/usr/share/rpcd/acl.d/luci-app-routeflux.json")"
remove_path "$(scope_path "/www/luci-static/resources/routeflux")"
remove_path "$(scope_path "/www/luci-static/resources/view/routeflux")"

remove_path "${xray_binary}"
remove_path "${xray_service}"
remove_path "${xray_config_dir}"
remove_path "${routeflux_cron_helper}"
remove_path "$(scope_path "/var/log/xray.log")"
remove_path "$(scope_path "/var/run/xray.pid")"

remove_glob "$(scope_path "/etc/rc.d/*routeflux")"
remove_glob "$(scope_path "/etc/rc.d/*xray")"
remove_glob "$(scope_path "/tmp/routeflux*")"
remove_glob "$(scope_path "/tmp/xray*")"
remove_glob "$(scope_path "/tmp/luci-indexcache*")"
remove_path "$(scope_path "/tmp/luci-modulecache")"

run_service_if_present "${rpcd_service}" reload || true
run_service_if_present "${uhttpd_service}" reload || true

log "RouteFlux and bundled Xray removed."
