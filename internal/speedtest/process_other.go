//go:build !linux

package speedtest

import "os/exec"

func configureProcess(cmd *exec.Cmd) {}

func terminateProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	return cmd.Process.Kill()
}
