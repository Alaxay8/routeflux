#!/bin/sh
set -eu

ROUTEFLUX_INSTALL_ROOT="${ROUTEFLUX_INSTALL_ROOT:-/}"
ROUTEFLUX_ROOT_OVERRIDE="${ROUTEFLUX_ROOT:-}"
ROUTEFLUX_XRAY_BINARY_PATH="${ROUTEFLUX_XRAY_BINARY:-}"
ROUTEFLUX_XRAY_SERVICE_PATH="${ROUTEFLUX_XRAY_SERVICE:-}"
ROUTEFLUX_XRAY_CONFIG_PATH="${ROUTEFLUX_XRAY_CONFIG:-}"
ROUTEFLUX_ZAPRET_SERVICE_PATH="${ROUTEFLUX_ZAPRET_SERVICE:-}"
ROUTEFLUX_ZAPRET_CONFIG_PATH="${ROUTEFLUX_ZAPRET_CONFIG:-}"
ROUTEFLUX_ZAPRET_CONFIG_BAK_PATH="${ROUTEFLUX_ZAPRET_CONFIG_BAK:-}"
ROUTEFLUX_ZAPRET_HOSTLIST_PATH="${ROUTEFLUX_ZAPRET_HOSTLIST:-}"
ROUTEFLUX_ZAPRET_HOSTLIST_BAK_PATH="${ROUTEFLUX_ZAPRET_HOSTLIST_BAK:-}"
ROUTEFLUX_ZAPRET_IPLIST_PATH="${ROUTEFLUX_ZAPRET_IPLIST:-}"
ROUTEFLUX_ZAPRET_IPLIST_BAK_PATH="${ROUTEFLUX_ZAPRET_IPLIST_BAK:-}"
ROUTEFLUX_ZAPRET_MARKER_PATH="${ROUTEFLUX_ZAPRET_MARKER:-}"
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

routeflux_self_update_helper_path() {
	scope_path "/usr/libexec/routeflux-self-update"
}

install_manifest_path() {
	printf '%s/install-manifest.txt\n' "$(routeflux_root_path)"
}

zapret_service_path() {
	path="${ROUTEFLUX_ZAPRET_SERVICE_PATH:-/etc/init.d/zapret}"
	scope_path "${path}"
}

zapret_config_path() {
	path="${ROUTEFLUX_ZAPRET_CONFIG_PATH:-/opt/zapret/config}"
	scope_path "${path}"
}

zapret_config_backup_path() {
	if [ -n "${ROUTEFLUX_ZAPRET_CONFIG_BAK_PATH}" ]; then
		scope_path "${ROUTEFLUX_ZAPRET_CONFIG_BAK_PATH}"
		return
	fi
	printf '%s/zapret-config.routeflux.bak\n' "$(routeflux_root_path)"
}

zapret_hostlist_path() {
	path="${ROUTEFLUX_ZAPRET_HOSTLIST_PATH:-/opt/zapret/ipset/zapret-hosts-user.txt}"
	scope_path "${path}"
}

zapret_hostlist_backup_path() {
	path="${ROUTEFLUX_ZAPRET_HOSTLIST_BAK_PATH:-/opt/zapret/ipset/zapret-hosts-user.txt.routeflux.bak}"
	scope_path "${path}"
}

zapret_iplist_path() {
	path="${ROUTEFLUX_ZAPRET_IPLIST_PATH:-/opt/zapret/ipset/zapret-ip-user.txt}"
	scope_path "${path}"
}

zapret_iplist_backup_path() {
	path="${ROUTEFLUX_ZAPRET_IPLIST_BAK_PATH:-/opt/zapret/ipset/zapret-ip-user.txt.routeflux.bak}"
	scope_path "${path}"
}

zapret_marker_path() {
	if [ -n "${ROUTEFLUX_ZAPRET_MARKER_PATH}" ]; then
		scope_path "${ROUTEFLUX_ZAPRET_MARKER_PATH}"
		return
	fi
	printf '%s/zapret-managed.json\n' "$(routeflux_root_path)"
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

command_exists() {
	command -v "$1" >/dev/null 2>&1
}

opkg_available() {
	command_exists opkg
}

manifest_values() {
	kind="$1"
	manifest="$2"

	[ -f "${manifest}" ] || return 0
	awk -F= -v kind="${kind}" '$1 == kind { print $2 }' "${manifest}"
}

manifest_has_matching_pkg() {
	prefix="$1"
	manifest="$2"

	[ -f "${manifest}" ] || return 1
	grep -Eq "^pkg=${prefix}($|[-_])" "${manifest}"
}

remove_manifest_packages() {
	manifest="$1"

	if [ ! -f "${manifest}" ] || ! opkg_available; then
		return 0
	fi

	manifest_values "pkg" "${manifest}" | awk '{ lines[NR] = $0 } END { for (i = NR; i >= 1; i--) print lines[i] }' | while IFS= read -r pkg; do
		[ -n "${pkg}" ] || continue
		log "Removing installer-managed package ${pkg}"
		opkg remove "${pkg}" || true
	done
}

restore_manifest_packages() {
	manifest="$1"

	if [ ! -f "${manifest}" ] || ! opkg_available; then
		return 0
	fi

	manifest_values "restore" "${manifest}" | while IFS= read -r pkg; do
		[ -n "${pkg}" ] || continue
		log "Restoring package ${pkg}"
		opkg install "${pkg}" || true
	done
}

cleanup_zapret_managed_file() {
	path="$1"
	backup="$2"

	if [ -f "${backup}" ]; then
		mkdir -p "$(dirname "${path}")"
		cp "${backup}" "${path}" || true
	else
		remove_path "${path}"
	fi

	remove_path "${backup}"
}

cleanup_zapret_state() {
	config="$1"
	config_backup="$2"
	hostlist="$3"
	hostlist_backup="$4"
	iplist="$5"
	iplist_backup="$6"
	marker="$7"

	cleanup_zapret_managed_file "${config}" "${config_backup}"
	cleanup_zapret_managed_file "${hostlist}" "${hostlist_backup}"
	cleanup_zapret_managed_file "${iplist}" "${iplist_backup}"
	remove_path "${marker}"
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
zapret_service="$(zapret_service_path)"
zapret_config="$(zapret_config_path)"
zapret_config_backup="$(zapret_config_backup_path)"
zapret_hostlist="$(zapret_hostlist_path)"
zapret_hostlist_backup="$(zapret_hostlist_backup_path)"
zapret_iplist="$(zapret_iplist_path)"
zapret_iplist_backup="$(zapret_iplist_backup_path)"
zapret_marker="$(zapret_marker_path)"
install_manifest="$(install_manifest_path)"
routeflux_cron_helper="$(routeflux_cron_helper_path)"
routeflux_self_update_helper="$(routeflux_self_update_helper_path)"
rpcd_service="$(scope_path "/etc/init.d/rpcd")"
uhttpd_service="$(scope_path "/etc/init.d/uhttpd")"
remove_installer_zapret=0

if [ "${ROUTEFLUX_DRY_RUN}" = "1" ]; then
	log "RouteFlux root: ${routeflux_root}"
	log "RouteFlux binary: ${routeflux_binary}"
	log "RouteFlux service: ${routeflux_service}"
	log "Xray binary: ${xray_binary}"
	log "Xray service: ${xray_service}"
	log "Xray config: ${xray_config}"
	log "Zapret service: ${zapret_service}"
	log "Zapret config: ${zapret_config}"
	log "Zapret hostlist: ${zapret_hostlist}"
	log "Zapret ip list: ${zapret_iplist}"
	log "Install manifest: ${install_manifest}"
	exit 0
fi

if manifest_has_matching_pkg "zapret" "${install_manifest}"; then
	remove_installer_zapret=1
fi

if [ -x "${routeflux_binary}" ]; then
	"${routeflux_binary}" --root "${routeflux_root}" disconnect >/dev/null 2>&1 || true
	"${routeflux_binary}" --root "${routeflux_root}" firewall disable >/dev/null 2>&1 || true
fi

run_service_if_present "${routeflux_service}" stop || true
run_service_if_present "${routeflux_service}" disable || true
run_service_if_present "${xray_service}" stop || true
run_service_if_present "${xray_service}" disable || true
run_service_if_present "${zapret_service}" stop || true
run_service_if_present "${zapret_service}" disable || true
ROUTEFLUX_INSTALL_ROOT="${ROUTEFLUX_INSTALL_ROOT}" "${routeflux_cron_helper}" remove-xray-log-retention >/dev/null 2>&1 || true
remove_manifest_packages "${install_manifest}"
restore_manifest_packages "${install_manifest}"
cleanup_zapret_state "${zapret_config}" "${zapret_config_backup}" "${zapret_hostlist}" "${zapret_hostlist_backup}" "${zapret_iplist}" "${zapret_iplist_backup}" "${zapret_marker}"

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
if [ "${remove_installer_zapret}" = "1" ]; then
	remove_path "${zapret_service}"
	remove_path "$(scope_path "/opt/zapret")"
fi
remove_path "${routeflux_cron_helper}"
remove_path "${routeflux_self_update_helper}"
remove_path "$(scope_path "/var/log/xray.log")"
remove_path "$(scope_path "/var/run/xray.pid")"

remove_glob "$(scope_path "/etc/rc.d/*routeflux")"
remove_glob "$(scope_path "/etc/rc.d/*xray")"
if [ "${remove_installer_zapret}" = "1" ]; then
	remove_glob "$(scope_path "/etc/rc.d/*zapret")"
fi
remove_glob "$(scope_path "/tmp/routeflux*")"
remove_glob "$(scope_path "/tmp/xray*")"
remove_glob "$(scope_path "/tmp/luci-indexcache*")"
remove_path "$(scope_path "/tmp/luci-modulecache")"

run_service_if_present "${rpcd_service}" reload || true
run_service_if_present "${uhttpd_service}" reload || true

log "RouteFlux, bundled Xray, Zapret, and installer-managed packages removed."
