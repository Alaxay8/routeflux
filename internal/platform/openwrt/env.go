package openwrt

import "os"

// IsOpenWrt returns true when the process appears to run on OpenWrt.
func IsOpenWrt() bool {
	if _, err := os.Stat("/etc/openwrt_release"); err == nil {
		return true
	}
	return false
}
