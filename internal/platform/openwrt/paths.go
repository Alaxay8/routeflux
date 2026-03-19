package openwrt

import (
	"os"
	"path/filepath"
)

const (
	defaultRoot       = "/etc/routeflux"
	defaultXrayConfig = "/etc/xray/config.json"
	defaultService    = "/etc/init.d/xray"
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
