package luci_test

import (
	"os/exec"
	"testing"
)

func TestProxyViewBehavior(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required for LuCI behavior tests")
	}
	cmd := exec.Command(node, "proxy_behavior_test.js")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("LuCI proxy behavior: %v\n%s", err, output)
	}
}
