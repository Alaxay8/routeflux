//go:build linux

package speedtest

import (
	"os/exec"
	"syscall"
	"testing"
)

func TestConfigureProcessSetsLinuxSafetyAttrs(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("echo", "ok")
	configureProcess(cmd)

	if cmd.SysProcAttr == nil {
		t.Fatal("expected SysProcAttr to be configured")
	}
	if !cmd.SysProcAttr.Setpgid {
		t.Fatal("expected Setpgid to be enabled")
	}
	if cmd.SysProcAttr.Pdeathsig != syscall.SIGKILL {
		t.Fatalf("expected Pdeathsig SIGKILL, got %v", cmd.SysProcAttr.Pdeathsig)
	}
}
