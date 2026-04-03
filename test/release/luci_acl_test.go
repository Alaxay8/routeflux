package release_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLuCIACLReadPermissionsUseSafeWhitelist(t *testing.T) {
	t.Parallel()

	path := filepath.Join(repoRoot(t), "luci-app-routeflux", "root", "usr", "share", "rpcd", "acl.d", "luci-app-routeflux.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ACL file: %v", err)
	}

	var payload struct {
		App struct {
			Read struct {
				File map[string][]string `json:"file"`
			} `json:"read"`
			Write struct {
				File map[string][]string `json:"file"`
			} `json:"write"`
		} `json:"luci-app-routeflux"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal ACL json: %v", err)
	}

	wantRead := map[string]struct{}{
		"/usr/bin/routeflux --json version":                                 {},
		"/usr/bin/routeflux --json status":                                  {},
		"/usr/bin/routeflux --json list subscriptions":                      {},
		"/usr/bin/routeflux inspect xray-safe --subscription * --node *":    {},
		"/usr/bin/routeflux --json inspect speed --subscription * --node *": {},
		"/usr/bin/routeflux --json settings get":                            {},
		"/usr/bin/routeflux --json dns get":                                 {},
		"/usr/bin/routeflux dns explain":                                    {},
		"/usr/bin/routeflux --json firewall get":                            {},
		"/usr/bin/routeflux firewall explain":                               {},
		"/usr/bin/routeflux --json services list":                           {},
		"/usr/bin/routeflux --json diagnostics":                             {},
		"/usr/bin/routeflux --json logs":                                    {},
	}

	if len(payload.App.Read.File) != len(wantRead) {
		t.Fatalf("unexpected read ACL entries:\nwant=%v\ngot=%v", wantRead, payload.App.Read.File)
	}
	for command, permissions := range payload.App.Read.File {
		if _, ok := wantRead[command]; !ok {
			t.Fatalf("unexpected read ACL command %q", command)
		}
		if len(permissions) != 1 || permissions[0] != "exec" {
			t.Fatalf("unexpected read ACL permissions for %q: %v", command, permissions)
		}
		if strings.Contains(command, "*") &&
			command != "/usr/bin/routeflux inspect xray-safe --subscription * --node *" &&
			command != "/usr/bin/routeflux --json inspect speed --subscription * --node *" {
			t.Fatalf("read ACL must not use wildcard command %q", command)
		}
		if strings.Contains(command, "inspect xray") && !strings.Contains(command, "inspect xray-safe") {
			t.Fatalf("read ACL must not expose raw inspect xray: %q", command)
		}
	}

	writePermissions, ok := payload.App.Write.File["/usr/bin/routeflux *"]
	if !ok {
		t.Fatalf("expected write ACL wildcard to remain, got %v", payload.App.Write.File)
	}
	if len(writePermissions) != 1 || writePermissions[0] != "exec" {
		t.Fatalf("unexpected write ACL permissions: %v", writePermissions)
	}
}
