package openwrt

import (
	"os"
	"path/filepath"
)

const (
	defaultRoot              = "/etc/routeflux"
	defaultXrayConfig        = "/etc/xray/config.json"
	defaultService           = "/etc/init.d/xray"
	defaultZapretService     = "/etc/init.d/zapret"
	defaultZapretConfig      = "/opt/zapret/config"
	defaultZapretHostlist    = "/opt/zapret/ipset/zapret-hosts-user.txt"
	defaultZapretHostlistBak = "/opt/zapret/ipset/zapret-hosts-user.txt.routeflux.bak"
	defaultZapretIPList      = "/opt/zapret/ipset/zapret-ip-user.txt"
	defaultZapretIPListBak   = "/opt/zapret/ipset/zapret-ip-user.txt.routeflux.bak"
)

// RootDir returns the RouteFlux state directory.
func RootDir() string {
	if root := os.Getenv("ROUTEFLUX_ROOT"); root != "" {
		return root
	}
	if IsOpenWrt() {
		return defaultRoot
	}
	return filepath.Join(".", ".routeflux")
}

// XrayConfigPath returns the default Xray config path.
func XrayConfigPath() string {
	if path := os.Getenv("ROUTEFLUX_XRAY_CONFIG"); path != "" {
		return path
	}
	if IsOpenWrt() {
		return defaultXrayConfig
	}
	return filepath.Join(RootDir(), "xray-config.json")
}

// XrayServicePath returns the init.d control script path.
func XrayServicePath() string {
	if path := os.Getenv("ROUTEFLUX_XRAY_SERVICE"); path != "" {
		return path
	}
	return defaultService
}

// ZapretServicePath returns the init.d control script path for zapret-openwrt.
func ZapretServicePath() string {
	if path := os.Getenv("ROUTEFLUX_ZAPRET_SERVICE"); path != "" {
		return path
	}
	return defaultZapretService
}

// ZapretConfigPath returns the live zapret-openwrt config path.
func ZapretConfigPath() string {
	if path := os.Getenv("ROUTEFLUX_ZAPRET_CONFIG"); path != "" {
		return path
	}
	return defaultZapretConfig
}

// ZapretHostlistPath returns the RouteFlux-managed user hostlist path.
func ZapretHostlistPath() string {
	if path := os.Getenv("ROUTEFLUX_ZAPRET_HOSTLIST"); path != "" {
		return path
	}
	return defaultZapretHostlist
}

// ZapretHostlistBackupPath returns the backup of the original user hostlist.
func ZapretHostlistBackupPath() string {
	if path := os.Getenv("ROUTEFLUX_ZAPRET_HOSTLIST_BAK"); path != "" {
		return path
	}
	return defaultZapretHostlistBak
}

// ZapretIPListPath returns the RouteFlux-managed user IP list path.
func ZapretIPListPath() string {
	if path := os.Getenv("ROUTEFLUX_ZAPRET_IPLIST"); path != "" {
		return path
	}
	return defaultZapretIPList
}

// ZapretIPListBackupPath returns the backup of the original user IP list.
func ZapretIPListBackupPath() string {
	if path := os.Getenv("ROUTEFLUX_ZAPRET_IPLIST_BAK"); path != "" {
		return path
	}
	return defaultZapretIPListBak
}

// ZapretMarkerPath returns the RouteFlux marker file for managed zapret fallback.
func ZapretMarkerPath() string {
	if path := os.Getenv("ROUTEFLUX_ZAPRET_MARKER"); path != "" {
		return path
	}
	return filepath.Join(RootDir(), "zapret-managed.json")
}

// ZapretConfigBackupPath returns the backup of the original zapret config.
func ZapretConfigBackupPath() string {
	if path := os.Getenv("ROUTEFLUX_ZAPRET_CONFIG_BAK"); path != "" {
		return path
	}
	return filepath.Join(RootDir(), "zapret-config.routeflux.bak")
}
