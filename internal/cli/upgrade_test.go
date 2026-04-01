package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpgradeFlagRunsLatestInstaller(t *testing.T) {
	binDir := t.TempDir()
	wgetLog := filepath.Join(binDir, "wget.log")
	shLog := filepath.Join(binDir, "sh.log")
	installerPath := filepath.Join(binDir, "routeflux-install.sh")

	writeExecutableScript(t, filepath.Join(binDir, "wget"), fmt.Sprintf(`#!/bin/sh
set -eu
log=%q
out=""
url=""
while [ "$#" -gt 0 ]; do
	case "$1" in
		-O)
			out="$2"
			shift 2
			;;
		*)
			url="$1"
			shift
			;;
	esac
done
printf 'out=%%s\nurl=%%s\n' "$out" "$url" > "$log"
cat > "$out" <<'EOF'
#!/bin/sh
printf 'installer script payload\n'
EOF
chmod 0755 "$out"
`, wgetLog))
	writeExecutableScript(t, filepath.Join(binDir, "sh"), fmt.Sprintf(`#!/bin/sh
set -eu
log=%q
printf 'script=%%s\n' "$1" > "$log"
printf 'installer completed\n'
`, shLog))

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	prevInstallerPath := routefluxUpgradeInstallerPath
	routefluxUpgradeInstallerPath = installerPath
	t.Cleanup(func() {
		routefluxUpgradeInstallerPath = prevInstallerPath
	})

	cmd := newRootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"--upgrade"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute upgrade flag: %v", err)
	}

	if got := stdout.String(); !strings.Contains(got, "installer completed") || !strings.Contains(got, "Upgrade completed using the latest release installer.") {
		t.Fatalf("unexpected upgrade stdout:\n%s", got)
	}

	wgetData, err := os.ReadFile(wgetLog)
	if err != nil {
		t.Fatalf("read wget log: %v", err)
	}
	for _, want := range []string{
		"out=" + installerPath,
		"url=" + routefluxLatestInstallScriptURL,
	} {
		if !strings.Contains(string(wgetData), want) {
			t.Fatalf("wget log missing %q\n%s", want, wgetData)
		}
	}

	shData, err := os.ReadFile(shLog)
	if err != nil {
		t.Fatalf("read sh log: %v", err)
	}
	if !strings.Contains(string(shData), "script="+installerPath) {
		t.Fatalf("unexpected sh log:\n%s", shData)
	}
}

func TestUpgradeFlagJSONOutputsStructuredResult(t *testing.T) {
	binDir := t.TempDir()
	installerPath := filepath.Join(binDir, "routeflux-install.sh")

	writeExecutableScript(t, filepath.Join(binDir, "wget"), `#!/bin/sh
set -eu
out=""
while [ "$#" -gt 0 ]; do
	case "$1" in
		-O)
			out="$2"
			shift 2
			;;
		*)
			shift
			;;
	esac
done
cat > "$out" <<'EOF'
#!/bin/sh
printf 'json installer payload\n'
EOF
chmod 0755 "$out"
printf 'download ok\n'
`)
	writeExecutableScript(t, filepath.Join(binDir, "sh"), `#!/bin/sh
set -eu
printf 'install ok\n'
`)

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	prevInstallerPath := routefluxUpgradeInstallerPath
	routefluxUpgradeInstallerPath = installerPath
	t.Cleanup(func() {
		routefluxUpgradeInstallerPath = prevInstallerPath
	})

	cmd := newRootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"--json", "--upgrade"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute upgrade flag json: %v", err)
	}

	var result upgradeResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal upgrade json: %v\n%s", err, stdout.String())
	}

	if result.Status != "ok" {
		t.Fatalf("unexpected upgrade status: %+v", result)
	}
	if result.URL != routefluxLatestInstallScriptURL {
		t.Fatalf("unexpected upgrade url: %+v", result)
	}
	if result.ScriptPath != installerPath {
		t.Fatalf("unexpected installer path: %+v", result)
	}
	if !strings.Contains(result.DownloadOutput, "download ok") {
		t.Fatalf("unexpected download output: %+v", result)
	}
	if !strings.Contains(result.InstallOutput, "install ok") {
		t.Fatalf("unexpected install output: %+v", result)
	}
}

func writeExecutableScript(t *testing.T, path string, contents string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}
